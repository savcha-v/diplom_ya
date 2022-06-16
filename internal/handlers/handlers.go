package handlers

import (
	"diplom_ya/internal/auth"
	"diplom_ya/internal/config"
	"diplom_ya/internal/cookie"
	"diplom_ya/internal/encryption"
	"diplom_ya/internal/store"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

func NewRouter(cfg config.Config) *chi.Mux {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Post("/api/user/register", userRegister(cfg)) // регистрация пользователя;
		r.Post("/api/user/login", userLogin(cfg))       // аутентификация пользователя;
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.CheckAuthorized(cfg))
		r.Post("/api/user/orders", postOrder(cfg))                  // загрузка пользователем номера заказа для расчёта;
		r.Get("/api/user/orders", getOrders(cfg))                   // получение списка загруженных пользователем номеров заказов, статусов их обработки и информации о начислениях;
		r.Get("/api/user/balance", getBalance(cfg))                 // получение текущего баланса счёта баллов лояльности пользователя;
		r.Post("/api/user/balance/withdraw", postWithdraw(cfg))     // запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа;
		r.Get("/api/user/balance/withdrawals", getWithdrawals(cfg)) // получение информации о выводе средств с накопительного счёта пользователем.
	})

	return r
}

func userRegister(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(os.Stdout, "userRegister")

		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		type in struct {
			Login string `json:"login"`
			Pass  string `json:"password"`
		}

		valueIn := in{}

		if err := json.Unmarshal(body, &valueIn); err != nil || valueIn.Login == "" || valueIn.Pass == "" {
			http.Error(w, "register unmarshal error", http.StatusBadRequest)
			return
		}

		use, err := auth.LoginUse(r.Context(), cfg, valueIn.Login)
		if err != nil {
			http.Error(w, "data base err", http.StatusInternalServerError)
			return
		}
		if use {
			http.Error(w, "login already in use", http.StatusConflict)
			return
		}

		userID, err := auth.NewUser(r.Context(), cfg, valueIn.Login, valueIn.Pass)
		if err != nil {
			http.Error(w, "data base err", http.StatusInternalServerError)
			return
		}

		cookie.AddCookie("userID", userID, w, r)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(""))
		fmt.Fprint(w)

	}
}

func userLogin(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(os.Stdout, "userLogin")

		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		type in struct {
			Login string `json:"login"`
			Pass  string `json:"password"`
		}

		valueIn := in{}

		if err := json.Unmarshal(body, &valueIn); err != nil || valueIn.Login == "" || valueIn.Pass == "" {
			http.Error(w, "login unmarshal error", http.StatusBadRequest)
			return
		}

		userID, err := auth.AuthorizeUser(r.Context(), cfg, valueIn.Login, valueIn.Pass)
		if err != nil {
			http.Error(w, "data base err", http.StatusInternalServerError)
			return
		}
		if userID == "" {
			http.Error(w, "invalid username/password pair", http.StatusUnauthorized)
			return
		}

		cookie.AddCookie("userID", userID, w, r)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(""))
		fmt.Fprint(w)
	}
}

func getBalance(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(os.Stdout, "getBalance")

		type out struct {
			Balanse  float32 `json:"current"`
			SumSpent float32 `json:"withdrawn"`
		}

		userID := cookie.GetCookie(r, cfg, "userID")

		balanse, spent, err := store.GetBalanseSpent(r.Context(), cfg, userID)
		if err != nil {
			http.Error(w, "data base error", http.StatusInternalServerError)
			return
		}

		valueOut := out{}
		valueOut.Balanse = balanse
		valueOut.SumSpent = spent

		result, err := json.Marshal(valueOut)
		if err != nil {
			http.Error(w, "marshal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(result)
		fmt.Fprint(w)
	}
}

func postOrder(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// 200 — номер заказа уже был загружен этим пользователем;
		// 202 — новый номер заказа принят в обработку;
		// 400 — неверный формат запроса;
		// 401 — пользователь не аутентифицирован;
		// 409 — номер заказа уже был загружен другим пользователем;
		// 422 — неверный формат номера заказа;
		// 500 — внутренняя ошибка сервера.

		fmt.Fprintln(os.Stdout, "postOrder")

		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		order := string(body)
		if order == "" {
			http.Error(w, "order not found", http.StatusBadRequest)
			return
		}

		if !encryption.CheckOrder(order) {
			http.Error(w, "invalid format order", http.StatusUnprocessableEntity)
			return
		}

		userID := cookie.GetCookie(r, cfg, "userID")
		httpStatus := store.AddOrder(r.Context(), cfg, order, userID)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(httpStatus)
		w.Write([]byte(""))
		fmt.Fprint(w)
	}
}

func getOrders(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(os.Stdout, "getOrders")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		userID := cookie.GetCookie(r, cfg, "userID")

		valueOut, err := store.GetAccum(r.Context(), cfg, userID)
		fmt.Fprintln(os.Stdout, err)
		if err != nil {
			http.Error(w, "getOrders/ data base error", http.StatusInternalServerError)
			return
		}

		if len(valueOut) == 0 {
			http.Error(w, "getOrders/ no orders", http.StatusNoContent)
			return
		}

		result, err := json.Marshal(valueOut)
		if err != nil {
			http.Error(w, "getOrders/ marshal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(result)
		fmt.Fprint(w)
	}
}

func postWithdraw(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(os.Stdout, "postWithdraw")

		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			fmt.Fprintln(os.Stdout, "postWithdraw/ err:")
			fmt.Fprintln(os.Stdout, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprintln(os.Stdout, "postWithdraw/ body: "+string(body))

		type in struct {
			Order string  `json:"order"`
			Sum   float32 `json:"sum"`
		}

		valueIn := in{}

		if err := json.Unmarshal(body, &valueIn); err != nil || valueIn.Order == "" || valueIn.Sum == 0 {
			http.Error(w, "unmarshal error", http.StatusBadRequest)
			return
		}

		if !encryption.CheckOrder(valueIn.Order) {
			http.Error(w, "invalid format order", http.StatusUnprocessableEntity)
			return
		}

		userID := cookie.GetCookie(r, cfg, "userID")
		httpStatus := store.WriteWithdraw(r.Context(), cfg, valueIn.Order, valueIn.Sum, userID)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(httpStatus)
		w.Write([]byte(""))
		fmt.Fprint(w)

	}
}

func getWithdrawals(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(os.Stdout, "getWithdrawals")

		userID := cookie.GetCookie(r, cfg, "userID")

		valueOut, err := store.GetWithdrawals(r.Context(), cfg, userID)
		if err != nil {
			http.Error(w, "data base error", http.StatusInternalServerError)
			return
		}

		if len(valueOut) == 0 {
			http.Error(w, "no orders", http.StatusNoContent)
			return
		}

		result, err := json.Marshal(valueOut)
		if err != nil {
			http.Error(w, "marshal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(result)
		fmt.Fprint(w)
	}
}
