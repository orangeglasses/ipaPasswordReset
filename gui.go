package main

import (
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
)

type guiHandlerType struct {
}

type enterPwData struct {
	Username string
	Token    string
}

func (h pwResetReqHandler) PresentPwResetForm(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	data := enterPwData{
		Username: mux.Vars(r)["username"],
		Token:    mux.Vars(r)["token"],
	}

	tmpl := template.Must(template.ParseFiles("enterPw.tmpl"))

	tmpl.Execute(w, data)
}

func (h pwResetReqHandler) PresentResetRequestForm(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	tmpl := template.Must(template.ParseFiles("enterRequest.tmpl"))

	tmpl.Execute(w, nil)
}
