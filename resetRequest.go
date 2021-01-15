package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tehwalris/go-freeipa/freeipa"
	"gopkg.in/gomail.v2"
)

type ackResetRequestData struct {
	Success    bool
	Username   string
	ErrMessage string
}

func (h pwResetReqHandler) HandleResetRequest(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("ackResetRequest.tmpl"))

	r.ParseForm()

	username := r.FormValue("username")
	templData := ackResetRequestData{
		Username:   username,
		Success:    false,
		ErrMessage: "General error",
	}

	defer func() {
		r.Body.Close()
		tmpl.Execute(w, templData)
	}()

	var ctx = context.Background()

	ipaResult, err := h.ipaClient.UserShow(&freeipa.UserShowArgs{}, &freeipa.UserShowOptionalArgs{UID: &username})
	if err != nil {
		log.Printf("Error looking up user %v. Error: %v", username, err)
		w.WriteHeader(http.StatusNotFound)
		templData.ErrMessage = "Error while looking up user in IPA"
		return
	}

	//	fmt.Println(ipaResult.Value)

	token, err := uuid.NewUUID()
	if err != nil {
		log.Println("Unable to generate UUID: ", err)
		templData.ErrMessage = fmt.Sprintf("Unable to generate token: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.redisClient.Set(ctx, username, token.String(), 5*time.Minute).Err()
	if err != nil {
		log.Println("Unable to store token in redis: ", err)
		templData.ErrMessage = fmt.Sprintf("Unable to store token: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Println("Token stored for user: ", username)

	userEmail := (*ipaResult.Result.Mail)[0]

	m := gomail.NewMessage()
	m.SetHeader("From", h.config.EmailFrom)
	m.SetHeader("To", userEmail)
	m.SetHeader("Subject", "Haas PW Reset ")
	m.SetBody("text/plain", fmt.Sprintf("http://%v/enterpw/%v/%v", r.Host, username, token.String()))
	if err = h.mailClient.DialAndSend(m); err != nil {
		log.Println("Unable to send mail: ", err)
		log.Println("token: ", token.String())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	templData.Success = true
}
