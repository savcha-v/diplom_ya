package cookie

import (
	"diplom_ya/internal/config"
	"diplom_ya/internal/encryption"
	"net/http"
)

func GetCookie(r *http.Request, cfg config.Config, name string) string {
	value := ""
	if cookie, err := r.Cookie(name); err == nil {
		value, err = encryption.Decrypt(cookie.Value, cfg)
		if err != nil {
			value = cookie.Value
		}
	}
	return value
}

func AddCookie(name string, value string, w http.ResponseWriter, r *http.Request) {
	newCookie := &http.Cookie{
		Name:  name,
		Value: value,
	}
	http.SetCookie(w, newCookie)
	r.AddCookie(newCookie)
}
