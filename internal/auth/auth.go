package auth

import (
	"context"
	"diplom_ya/internal/config"
	"diplom_ya/internal/cookie"
	"diplom_ya/internal/encryption"
	"diplom_ya/internal/store"
	"fmt"
	"net/http"
)

func CheckAuthorized(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// получим куки для идентификации пользователя

			userID := cookie.GetCookie(r, cfg, "userID")
			if userID == "" {
				// no cookie
				fmt.Println("user not authorized0")
				http.Error(w, "user not authorized", http.StatusUnauthorized)
				return
			}

			exist, err := store.ExistsUserID(r.Context(), cfg, userID)
			if err != nil {
				// error server
				fmt.Println("data base err")
				http.Error(w, "data base err", http.StatusInternalServerError)
				return
			}
			if !exist {
				// no in data base
				fmt.Println("user not authorized1")
				http.Error(w, "user not authorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func LoginUse(ctx context.Context, cfg config.Config, login string) (bool, error) {
	use, err := store.LoginUse(ctx, cfg, login)
	return use, err
}

func NewUser(ctx context.Context, cfg config.Config, login string, pass string) (string, error) {
	// create hash
	msg := login + pass
	hash := encryption.Encrypt(msg, cfg)

	// write in db login/hash
	userID, err := store.WriteNewUser(ctx, cfg, login, hash)
	if err != nil {
		return "", err
	}
	// return userID
	return userID, nil
}

func AuthorizeUser(ctx context.Context, cfg config.Config, login string, pass string) (string, error) {
	// create hash
	msg := login + pass
	hash := encryption.Encrypt(msg, cfg)

	// read in db login/hash
	userID, err := store.ReadUser(ctx, cfg, login, hash)
	if err != nil {
		return "", err
	}
	// return userID
	return userID, nil
}
