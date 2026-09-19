package handlers

import (
	_ "embed"
	"net/http"
)

// adminHTML — статическая страница админки, вшивается в бинарник при сборке.
//
//go:embed admin.html
var adminHTML []byte

// AdminPage отдаёт HTML админ-панели (публичная страница; данные тянет из защищённого API по JWT).
func (h *Handlers) AdminPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(adminHTML)
}
