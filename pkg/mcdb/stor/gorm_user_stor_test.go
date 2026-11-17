package stor

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&mcmodel.User{}, &mcmodel.UserApiToken{})
	require.NoError(t, err)

	return db
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func TestGetUserByAPIToken_Success(t *testing.T) {
	db := setupTestDB(t)
	userStor := NewGormUserStor(db)

	user := mcmodel.User{
		ID:        1,
		UUID:      "user-uuid-1",
		Slug:      "testuser",
		Name:      "Test User",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := db.Create(&user).Error
	require.NoError(t, err)

	rawToken := "my-secret-api-token-12345"
	apiToken := mcmodel.UserApiToken{
		ID:        1,
		UUID:      "token-uuid-1",
		Name:      "Default Token",
		Token:     hashToken(rawToken),
		OwnerID:   user.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = db.Create(&apiToken).Error
	require.NoError(t, err)

	foundUser, err := userStor.GetUserByAPIToken(rawToken)
	require.NoError(t, err)
	require.NotNil(t, foundUser)
	require.Equal(t, user.ID, foundUser.ID)
	require.Equal(t, user.Email, foundUser.Email)
	require.Equal(t, user.Name, foundUser.Name)
}

func TestGetUserByAPIToken_NotFound(t *testing.T) {
	db := setupTestDB(t)
	userStor := NewGormUserStor(db)

	foundUser, err := userStor.GetUserByAPIToken("non-existent-token")
	require.Error(t, err)
	require.Nil(t, foundUser)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGetUserByAPIToken_EmptyToken(t *testing.T) {
	db := setupTestDB(t)
	userStor := NewGormUserStor(db)

	foundUser, err := userStor.GetUserByAPIToken("")
	require.Error(t, err)
	require.Nil(t, foundUser)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGetUserByAPIToken_MultipleUsersAndTokens(t *testing.T) {
	db := setupTestDB(t)
	userStor := NewGormUserStor(db)

	user1 := mcmodel.User{
		ID:    1,
		UUID:  "user-1",
		Name:  "User One",
		Email: "user1@example.com",
	}
	user2 := mcmodel.User{
		ID:    2,
		UUID:  "user-2",
		Name:  "User Two",
		Email: "user2@example.com",
	}
	require.NoError(t, db.Create(&user1).Error)
	require.NoError(t, db.Create(&user2).Error)

	rawTokenUser1A := "user-1-token-a"
	rawTokenUser1B := "user-1-token-b"
	rawTokenUser2 := "user-2-token"

	tokens := []mcmodel.UserApiToken{
		{
			ID:      1,
			UUID:    "token-1a",
			Name:    "Token 1A",
			Token:   hashToken(rawTokenUser1A),
			OwnerID: user1.ID,
		},
		{
			ID:      2,
			UUID:    "token-1b",
			Name:    "Token 1B",
			Token:   hashToken(rawTokenUser1B),
			OwnerID: user1.ID,
		},
		{
			ID:      3,
			UUID:    "token-2",
			Name:    "Token 2",
			Token:   hashToken(rawTokenUser2),
			OwnerID: user2.ID,
		},
	}
	for _, tok := range tokens {
		require.NoError(t, db.Create(&tok).Error)
	}

	// Token 1A returns User 1
	res1, err := userStor.GetUserByAPIToken(rawTokenUser1A)
	require.NoError(t, err)
	require.NotNil(t, res1)
	require.Equal(t, user1.ID, res1.ID)

	// Token 1B returns User 1
	res2, err := userStor.GetUserByAPIToken(rawTokenUser1B)
	require.NoError(t, err)
	require.NotNil(t, res2)
	require.Equal(t, user1.ID, res2.ID)

	// Token 2 returns User 2
	res3, err := userStor.GetUserByAPIToken(rawTokenUser2)
	require.NoError(t, err)
	require.NotNil(t, res3)
	require.Equal(t, user2.ID, res3.ID)

	// Using the hashed token directly fails because GetUserByAPIToken hashes its input
	_, err = userStor.GetUserByAPIToken(hashToken(rawTokenUser1A))
	require.Error(t, err)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGetUserByAPIToken_SpecialCharactersAndLength(t *testing.T) {
	db := setupTestDB(t)
	userStor := NewGormUserStor(db)

	user := mcmodel.User{
		ID:    1,
		UUID:  "user-special",
		Name:  "Special User",
		Email: "special@example.com",
	}
	require.NoError(t, db.Create(&user).Error)

	specialToken := "token_with_!@#$%^&*()_+=-~`{}[]|:;'<>,.?/ and spaces and unicode: 🚀 materials"
	apiToken := mcmodel.UserApiToken{
		ID:      1,
		UUID:    "tok-special",
		Name:    "Special Token",
		Token:   hashToken(specialToken),
		OwnerID: user.ID,
	}
	require.NoError(t, db.Create(&apiToken).Error)

	foundUser, err := userStor.GetUserByAPIToken(specialToken)
	require.NoError(t, err)
	require.NotNil(t, foundUser)
	require.Equal(t, user.ID, foundUser.ID)
}

func TestGetUserByAPIToken_RequiresHashedTokenInDB(t *testing.T) {
	db := setupTestDB(t)
	userStor := NewGormUserStor(db)

	user := mcmodel.User{
		ID:    1,
		UUID:  "user-plain",
		Name:  "Plain User",
		Email: "plain@example.com",
	}
	require.NoError(t, db.Create(&user).Error)

	rawToken := "unhashed-raw-token"
	// Store unhashed token directly in DB
	apiToken := mcmodel.UserApiToken{
		ID:      1,
		UUID:    "tok-unhashed",
		Name:    "Unhashed Token",
		Token:   rawToken,
		OwnerID: user.ID,
	}
	require.NoError(t, db.Create(&apiToken).Error)

	// Querying with rawToken fails because GetUserByAPIToken hashes it before querying
	foundUser, err := userStor.GetUserByAPIToken(rawToken)
	require.Error(t, err)
	require.Nil(t, foundUser)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormUserStor_OtherMethods(t *testing.T) {
	db := setupTestDB(t)
	userStor := NewGormUserStor(db)

	// CreateUser
	newUser := &mcmodel.User{
		Slug:       "jdoe",
		Name:       "John Doe",
		Email:      "jdoe@example.com",
		GlobusUser: "jdoe@globusid.org",
	}
	created, err := userStor.CreateUser(newUser)
	require.NoError(t, err)
	require.NotEmpty(t, created.UUID)

	// GetUserBySlug
	bySlug, err := userStor.GetUserBySlug("jdoe")
	require.NoError(t, err)
	require.Equal(t, "jdoe@example.com", bySlug.Email)

	// GetUserByEmail
	byEmail, err := userStor.GetUserByEmail("jdoe@example.com")
	require.NoError(t, err)
	require.Equal(t, "jdoe", byEmail.Slug)

	// GetUsersWithGlobusAccount
	globusUsers, err := userStor.GetUsersWithGlobusAccount()
	require.NoError(t, err)
	require.Len(t, globusUsers, 1)
	require.Equal(t, "jdoe@globusid.org", globusUsers[0].GlobusUser)
}
