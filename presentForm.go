package main

import (
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
)

func (h pwResetReqHandler) PresentPwResetForm(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	data := struct {
		Username    string
		Token       string
		AppName     string
		MinPwLength int
	}{
		Username:    mux.Vars(r)["username"],
		Token:       mux.Vars(r)["token"],
		AppName:     h.config.AppName,
		MinPwLength: h.config.MinPasswordLength,
	}

	tmpl := template.Must(template.ParseFiles("enterPw.tmpl"))

	tmpl.Execute(w, data)
}

func (h pwResetReqHandler) PresentResetRequestForm(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	tmpl := template.Must(template.ParseFiles("enterRequest.tmpl"))

	data := struct {
		AppName string
	}{AppName: h.config.AppName}

	tmpl.Execute(w, data)
}
