package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tehwalris/go-freeipa/freeipa"
	"golang.org/x/net/context"
	"gopkg.in/gomail.v2"
)

type confirmResBody struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h pwResetReqHandler) HandleConfirmRequest(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("ackReset.tmpl"))

	r.ParseForm()
	reqUsername := r.FormValue("username")
	reqToken := r.FormValue("token")
	reqPassword := r.FormValue("password")

	templData := ackResetRequestData{
		Success:    false,
		Username:   reqUsername,
		ErrMessage: "General error",
	}

	defer func() {
		r.Body.Close()
		tmpl.Execute(w, templData)
	}()

	ctx := context.Background()

	//get token
	token, err := h.redisClient.Get(ctx, reqUsername).Result()
	if err != nil {
		templData.ErrMessage = "Invalid username or token"
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//Check token matches username
	if token != reqToken {
		templData.ErrMessage = "Invalid username or token"
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//get user for e-mail adress
	ipaUserResult, err := h.ipaClient.UserShow(&freeipa.UserShowArgs{}, &freeipa.UserShowOptionalArgs{UID: &reqUsername})
	if err != nil {
		log.Printf("Error looking up user %v. Error: %v", reqUsername, err)
		w.WriteHeader(http.StatusNotFound)
		templData.ErrMessage = "Error while looking up user in IPA"
		return
	}
	userEmail := (*ipaUserResult.Result.Mail)[0]

	//Reset PW
	_, err = h.ipaClient.Passwd(&freeipa.PasswdArgs{Principal: reqUsername, Password: reqPassword}, &freeipa.PasswdOptionalArgs{})
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Printf("Password for user %s successfully reset\n", reqUsername)

	//Set expiration date
	expDate := time.Now().In(time.UTC).Truncate(time.Second).AddDate(0, 3, 0)

	_, err = h.ipaClient.UserMod(&freeipa.UserModArgs{}, &freeipa.UserModOptionalArgs{UID: &reqUsername, Krbpasswordexpiration: &expDate})
	if err != nil && !strings.HasPrefix(err.Error(), "unexpected value for field Krbpasswordexpiration") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Printf("Expiration date for user %s successfully set to %v\n", reqUsername, expDate)

	//Send confirmation mail
	m := gomail.NewMessage()
	m.SetHeader("From", h.config.EmailFrom)
	m.SetHeader("To", userEmail)
	m.SetHeader("Subject", "Haas PW Reset Completed")
	m.SetBody("text/plain", "Your password was reset")
	if err = h.mailClient.DialAndSend(m); err != nil {
		log.Println("Unable to send mail: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//Delete token from Redis
	if err := h.redisClient.Del(ctx, reqUsername).Err(); err != nil {
		log.Println("Error deleting token from redis: ", err)
	}

	templData.Success = true
}
