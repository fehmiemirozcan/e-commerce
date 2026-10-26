package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Böyle bir e-posta veritabanında var mı ?
func CheckDuplicateEmail(ctx context.Context, db *pgxpool.Pool, email string) (bool, error) {

	var isEmailTaken bool

	query := `
	SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
	`

	queryRowError := db.QueryRow(ctx, query, email).Scan(&isEmailTaken)
	if queryRowError != nil {
		return false, queryRowError
	}

	return isEmailTaken, nil
}

// CreateUser, yeni bir kullanıcıyı veritabanına ekler ve oluşan ID'yi döndürür
func CreateUser(ctx context.Context, db *pgxpool.Pool, name, email, hashedPassword string) (int, error) {
	var newID int

	query := `
		INSERT INTO users (name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, 'customer', NOW(), NOW())
		RETURNING id;
	`
	// Veritabanına bir sorgu gönderip geriye sadece bir satır cevap beklediğimizde kullanılır
	// Scan: Dönen 'id' değerini newID değişkeninin hafıza adresine yaz
	queryRowError := db.QueryRow(ctx, query, name, email, hashedPassword).Scan(&newID)
	if queryRowError != nil {
		return 0, queryRowError
	}
	return newID, nil
}

// CreateVerificationToken, kullanıcı için e-posta doğrulama token'ını veritabanına kaydeder.
func CreateVerificationToken(ctx context.Context, db *pgxpool.Pool, userID int, token string, expires time.Time) error {

	query := `
	INSERT INTO email_verification_tokens (user_id,token,expires_at) VALUES ($1,$2,$3)
	`

	_, execError := db.Exec(ctx, query, userID, token, expires)
	if execError != nil {
		return execError
	}

	return nil
}

// VerifyEmailToken, verilen token'ı veritabanında arar ve geçerliyse kullanıcıyı onaylar.
func VerifyEmailToken(ctx context.Context, db *pgxpool.Pool, token string) error {
	// ard arda yapacağıız işlemler var (Transaction) hepsi tamamlanana kadar bekle
	tx, beginError := db.Begin(ctx)
	if beginError != nil {
		return fmt.Errorf("Transaction could not be started/İşlem başlatılamadı:%w", beginError)
	}
	// Fonksiyon bittiğinde comit edilmemeişse iptal et
	defer tx.Rollback(ctx)

	var userId int

	// Token geçerli mi diye bakıp kime ait olduğunu buluyorum.
	querRowError := tx.QueryRow(ctx, `
	SELECT user_id FROM email_verification_tokens
	WHERE token = $1 AND expires_at > NOw()`, token).Scan(&userId)
	// expires_at > NOw():Son kullanma tarihi (expires_at), şu Anki zamandan (NOW()) daha ileri bir tarih olan
	if querRowError != nil {
		return fmt.Errorf("Invalid or expired token/Geçersiz veya süresi dolmuş token :%w", querRowError)
	}

	//Kullanıcının e-postasını onayladığı tam tarihi ve saati veritabanına yazıyoruz.
	_, execError := tx.Exec(ctx, `UPDATE users
	SET email_verified_at = NOW(), updated_at = NOW()
	WHERE id = $1`, userId)
	if execError != nil {
		return fmt.Errorf("User could not be updated/kullanici guncellenemedi: %w", execError)
	}

	// Kullanılan token'ı siliyoruz
	if _, execErr := tx.Exec(ctx, `
	DELETE FROM email_verification_tokens
	WHERE token = $1`, token); execErr != nil {
		return fmt.Errorf("Token could not be deleted/token silinemedi: %w", execErr)
	}

	// Az önce sırayla yaptığımz işlemlerde bir patlak yoksa veritabanına kaydet
	commitError := tx.Commit(ctx)
	if commitError != nil {
		return fmt.Errorf("The operation could not be saved/islem kaydedilemedi: %w", commitError)
	}
	return nil
}

type User struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
}

// GetUserByEmail, verilen e-postaya ait kullanıcı bilgilerini veritabanından getirir (Login işlemi için)
func GetUserByEmail(ctx context.Context, db *pgxpool.Pool, email string) (*User, error) {
	var user User

	query := `SELECT id, name, email, password_hash, role FROM users WHERE email = $1`

	queryRowError := db.QueryRow(ctx, query, email).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role)
	if queryRowError != nil {
		return nil, queryRowError
	}

	return &user, nil
}

// GetUserByID, verilen ID'ye ait kullanıcı bilgilerini veritabanından getirir
func GetUserByID(ctx context.Context, db *pgxpool.Pool, id int) (*User, error) {
	var user User

	query := `SELECT id, name, email, password_hash, role FROM users WHERE id = $1`

	queryRowError := db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role)
	if queryRowError != nil {
		return nil, queryRowError
	}

	return &user, nil
}

// CreatePasswordResetToken, password_reset_tokens tablosuna yeni bir token kaydeder.
func CreatePasswordResetToken(ctx context.Context, db *pgxpool.Pool, userID int, token string, expiresAt time.Time) error {

	// Aynı kullanıcı için önceki kullanılmamış token'ları sil (temizlik)
	_, _ = db.Exec(ctx, `DELETE FROM password_reset_tokens WHERE user_id = $1 AND used_at IS NULL`, userID)

	query := `
	INSERT INTO password_reset_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)
	`

	_, execError := db.Exec(ctx, query, userID, token, expiresAt)
	if execError != nil {
		return execError
	}

	return nil
}

// VerifyPasswordResetToken, password_reset_tokens tablosunda token'ı doğrular.
// Geçerli bir token bulunursa kullanıcı ID'sini döner ve token'ı kullanılmış olarak işaretler.
func VerifyPasswordResetToken(ctx context.Context, db *pgxpool.Pool, token string) (int, error) {

	tx, beginError := db.Begin(ctx)
	if beginError != nil {
		return 0, fmt.Errorf("Transaction could not be started/İşlem başlatılamadı: %w", beginError)
	}
	defer tx.Rollback(ctx)

	var userID int

	// Token geçerli mi? (süresi dolmamış ve kullanılmamış)
	queryRowError := tx.QueryRow(ctx, `
	SELECT user_id FROM password_reset_tokens
	WHERE token = $1 AND expires_at > NOW() AND used_at IS NULL`, token).Scan(&userID)
	if queryRowError != nil {
		return 0, fmt.Errorf("Invalid or expired token/Geçersiz veya süresi dolmuş token: %w", queryRowError)
	}

	// Token'ı kullanılmış olarak işaretle
	_, execError := tx.Exec(ctx, `
	UPDATE password_reset_tokens SET used_at = NOW() WHERE token = $1`, token)
	if execError != nil {
		return 0, fmt.Errorf("Token could not be marked as used/Token kullanılmış olarak işaretlenemedi: %w", execError)
	}

	commitError := tx.Commit(ctx)
	if commitError != nil {
		return 0, fmt.Errorf("The operation could not be saved/İşlem kaydedilemedi: %w", commitError)
	}

	return userID, nil
}

// UpdateUserPassword, kullanıcının şifresini günceller.
func UpdateUserPassword(ctx context.Context, db *pgxpool.Pool, userID int, hashedPassword string) error {

	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`

	_, execError := db.Exec(ctx, query, hashedPassword, userID)
	if execError != nil {
		return fmt.Errorf("Password could not be updated/Şifre güncellenemedi: %w", execError)
	}

	return nil
}

// CreateAdmin sadece test amaçlı admin oluşturmak için kullanılıyor.
func CreateAdmin(ctx context.Context, db *pgxpool.Pool, name, email, password string) (int, error) {
	query := `INSERT INTO users (name, email, password_hash, role, created_at, updated_at) VALUES ($1, $2, $3, 'admin', NOW(), NOW()) RETURNING id;`

	var newID int
	queryRowError := db.QueryRow(ctx, query, name, email, password).Scan(&newID)
	if queryRowError != nil {
		return 0, queryRowError
	}
	return newID, nil
}

