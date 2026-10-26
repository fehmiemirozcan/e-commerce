package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Racemir/e-commerce/internal/auth"
	"github.com/Racemir/e-commerce/internal/database"
	"github.com/Racemir/e-commerce/internal/mail"
	"github.com/Racemir/e-commerce/internal/server"
	"github.com/charmbracelet/x/term"

	gracefulshutdown "github.com/quii/go-graceful-shutdown"
	"golang.org/x/crypto/bcrypt"
)

func main() {

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Eğer dışarıdan adres gelmezse varsayılan adresi kullan (Geliştirme ortamı için hayat kurtarır)
		dbURL = "postgres://admin:secretpassword@localhost:5433/ecommerce?sslmode=disable"
	}

	// Veritabanı ile uygulama arasında bir bağlantı havuzu oluşturur.
	// Her istekte yeni bağlantı açmak yerine bu havuzdaki hazır bağlantılar kullanılır.
	dbPool, connectionPoolErr := database.ConnectionPool(dbURL)
	if connectionPoolErr != nil {
		log.Fatalf("Error ConnectionPool: %v", connectionPoolErr)
	}
	// Veritabanı bağlantılarının güvenlice kapatılması
	defer dbPool.Close()

	rdb, connectionRedisError := database.ConnectionRedis()
	if connectionRedisError != nil {
		log.Fatalf("Error connectionRedis: %v", connectionRedisError)
	}
	defer rdb.Close()

	// Argüman kontolü
	if len(os.Args) < 2 {
		fmt.Println("Kullanım: go run main.go [server|migrate|migrate-down]")
		return
	}

	// /api/health, / gibi yolların bağlı olduğu yönlendiriciyi (mux) alıyoruz.
	switch os.Args[1] {
	case "server":
		mux := server.SetupRoutes(dbPool, rdb)
		fmt.Println("Server 8080 portunda çalışıyor")

		// Kapanma sürecini yöneten context
		ctx := context.Background()

		// Standart GO HTTP sunucusu
		httpServer := &http.Server{
			Addr:    ":8080",
			Handler: mux,
		}

		// Graceful shutdown kütüphanesinden yeni bir sunucu oluşturur.
		gserver := gracefulshutdown.NewServer(httpServer)

		// Sunucuyu başlatır ve kapanma sinyallerini dinlemeye başlar.
		listenAndServeErr := gserver.ListenAndServe(ctx)

		if listenAndServeErr != nil {
			log.Fatalf("An error occurred while the server was shutting down (some requests may have been left incomplete)/Sunucu kapanırken hata oluştu (bazı istekler yarım kalmış olabilir): %v", listenAndServeErr)
		}
		fmt.Println("The server was shut down safely/Sunucu güvenli bir şekilde kapatıldı")

	case "migrate":

		runMigrationsUpError := database.RunMigrationsUp(dbURL)
		if runMigrationsUpError != nil {
			log.Fatalf("Migration up error/Migration up hatası: %v", runMigrationsUpError)
		}

		fmt.Println("The migration process was completed successfully/Migration işlemi başarıyla tamamlandı.")

	case "migrate-down":

		runMigrationsDownError := database.RunMigrationsDown(dbURL)
		if runMigrationsDownError != nil {
			log.Fatalf("Migration down error/Migration down hatası: %v", runMigrationsDownError)
		}
		fmt.Println("The system was successfully restored/Sistem başarıyla geri alındı")
	case "seed":
		fmt.Println("Seed feature is not yet configured / Seed özelliği henüz yapılandırılmadı")
	case "worker":
		fmt.Println("Starting worker/İşçi başlatılıyor")

		// Üstte (satır 35) zaten rdb bağlantısı açıldı, onu kullanıyoruz
		ctx := context.Background()

		for {
			// BRPop: Redis kuyruğunun sağından bir eleman çeker.
			// 0 = süresiz bekle (yeni iş gelene kadar bloklanır)
			result := rdb.BRPop(ctx, 0, "email_queue")

			// Adım 1: Hata kontrolü
			// Redis bağlantısı koparsa veya başka bir sorun olursa burada yakalarız
			if result.Err() != nil {
				log.Println("Redis BRPop hatası:", result.Err())
				continue // Hata olsa bile döngü devam etsin, worker ölmesin
			}

			// result.Val() bize []string döner: ["email_queue", "{json verisi}"]
			// [0] = kuyruk adı, [1] = asıl veri (bizim JSON'umuz)
			jobJSON := result.Val()[1]

			// JSON'u Go struct'ına çeviriyoruz (Unmarshal)
			// queue.go'da tanımladığın EmailQueuePayload struct'ını kullanıyoruz
			var payload auth.EmailQueuePayload
			unmarshalErr := json.Unmarshal([]byte(jobJSON), &payload)
			if unmarshalErr != nil {
				log.Println("JSON parse hatası:", unmarshalErr)
				continue
			}

			log.Printf("İş alındı — Tip: %s, Email: %s\n", payload.Type, payload.Email)

			// İşin tipine göre e-posta gönder
			if payload.Type == "send_verification_email" {
				// Doğrulama linkini oluştur
				verificationLink := fmt.Sprintf("http://localhost:8080/api/auth/verify-email?token=%s", payload.Token)

				subject := "E-posta Doğrulama / Email Verification"
				body := fmt.Sprintf("Merhaba,\n\nE-postanızı doğrulamak için bu linke tıklayın:\n%s", verificationLink)

				// mail.NewMailSender ile bir gönderici oluştur, sonra SendEmail ile gönder
				sender := mail.NewMailSender("localhost", "1025", "", "", "noreply@ecommerce.com")
				sendErr := sender.SendEmail(payload.Email, subject, body)
				if sendErr != nil {
					log.Println("E-posta gönderilemedi:", sendErr)
					continue
				}

				log.Printf("Doğrulama e-postası gönderildi: %s\n", payload.Email)
			}

			if payload.Type == "send_password_reset_email" {
				verificationLink := fmt.Sprintf("http://localhost:8080/api/auth/reset-password?token=%s", payload.Token)

				subject := "Şifre Sıfırlama / Password Reset"
				body := fmt.Sprintf("Merhaba,\n\nŞifrenizi sıfırlamak için bu linke tıklayın:\n%s", verificationLink)

				// mail.NewMailSender ile bir gönderici oluştur, sonra SendEmail ile gönder
				sender := mail.NewMailSender("mailpit", "1025", "", "", "noreply@ecommerce.com")
				sendErr := sender.SendEmail(payload.Email, subject, body)
				if sendErr != nil {
					log.Println("E-posta gönderilemedi:", sendErr)
					continue
				}

				log.Printf("Şifre sıfırlama e-postası gönderildi: %s\n", payload.Email)
			}
		}

	case "create-admin":
		fmt.Println("Creating admin account / Admin hesabı oluşturuluyor")
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Name: ")
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)

		fmt.Print("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		fmt.Print("Password: ")
		bytePassword, readPasswordError := term.ReadPassword(os.Stdin.Fd())
		if readPasswordError != nil {
			log.Fatalf("\nAn error occurred while reading the password./Şifre okunurken bir hata oluştu")
		}
		password := string(bytePassword)
		hashedPassword, passwordError := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if passwordError != nil {
			log.Fatalf("An error occurred while encrypting the password/Şifre şifrenirken hata oluştu")
		}
		ctx := context.Background()
		auth.CreateAdmin(ctx, dbPool, name, email, string(hashedPassword))
	default:
		log.Fatalf("Unknown command")
	}
}
