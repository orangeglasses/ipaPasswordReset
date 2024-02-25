package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vchrisr/go-freeipa/freeipa"
	"gopkg.in/gomail.v2"
)

func (h pwResetReqHandler) userInBlockedGroup(memberOf *[]string) bool {
	if memberOf == nil {
		return false
	}

	m := *memberOf
	for _, grp := range m {
		if _, ok := h.BlockedGroups[grp]; ok {
			return true
		}
	}

	return false
}

func (h pwResetReqHandler) userInBlockedPrefixes(username string) bool {
	for _, prefix := range h.config.BlockedPrefixes {
		if strings.HasPrefix(username, prefix) {
			return true
		}
	}
	return false
}

func (h pwResetReqHandler) userInSvcAccountPrefixes(username string) bool {
	for _, prefix := range h.config.ServiceAccountPrefixes {
		if strings.HasPrefix(username, prefix) {
			return true
		}
	}
	return false
}

func (h pwResetReqHandler) getUserMail(user freeipa.User) (string, []string) {
	var mailCC []string

	userEmail := (*user.Mail)[0]

	if len((*user.Mail)) > 1 {
		mailCC = (*user.Mail)[1:]
	}

	return userEmail, mailCC
}

func (h pwResetReqHandler) sendMail(to string, cc []string, subject, msg string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", h.config.EmailFrom)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", msg)
	if len(cc) > 0 {
		m.SetHeader("Cc", cc...)
	}

	if err := h.mailClient.DialAndSend(m); err != nil {
		log.Println("Unable to send mail: ", err)
		return err
	}

	return nil
}

func (h pwResetReqHandler) HandleResetRequest(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("ackResetRequest.tmpl"))

	r.ParseForm()

	username := r.FormValue("username")
	templData := struct {
		Success    bool
		Username   string
		ErrMessage string
		Expire     int
		AppName    string
	}{
		Username:   username,
		Success:    false,
		ErrMessage: "General error",
		Expire:     h.config.TokenValidity,
	}

	defer func() {
		r.Body.Close()
		tmpl.Execute(w, templData)
	}()

	addDefaultHeaders(&w)
	var ctx = context.Background()

	ipaResult, err := h.ipaClient.UserShow(&freeipa.UserShowArgs{}, &freeipa.UserShowOptionalArgs{UID: &username})
	if err != nil {
		log.Printf("Error looking up user %v. Error: %v\n", username, err)
		templData.Success = true
		return
	}

	blocked := h.userInBlockedGroup(ipaResult.Result.MemberofGroup)
	blockedByPrefix := h.userInBlockedPrefixes(username)
	userEmail, mailCC := h.getUserMail(ipaResult.Result)
	DontEnableButLocked := (!h.config.IpaEnableAccountOnReset && *ipaResult.Result.Nsaccountlock) //account enabling not allowed but current account is locked

	if blocked || blockedByPrefix || DontEnableButLocked {
		log.Printf("User %s is locked, member of a blocked group, or blocked prefix\n", username)
		h.sendMail(userEmail, mailCC, "Password reset request denied", "Thank you for using this service to request a password reset. Unfortunately I am not allowed to reset your password as the given account matches one of these conditions: Account is locked, Account is member of a blocked group Or account name has a specific prefix. Please contact your admin.")
		templData.Success = true
		return
	}

	token, err := uuid.NewUUID()
	if err != nil {
		log.Println("Unable to generate UUID: ", err)
		templData.ErrMessage = fmt.Sprintf("Unable to generate token: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.redisClient.Set(ctx, username, token.String(), time.Duration(h.config.TokenValidity)*time.Minute).Err()
	if err != nil {
		log.Println("Unable to store token in redis: ", err)
		templData.ErrMessage = fmt.Sprintf("Unable to store token. Please contact you system admin if this problem persists.")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println("Token stored for user: ", username)

	confirmLink := fmt.Sprintf("https://%v/enterpw/%v/%v", r.Host, username, token.String())

	if err = h.sendMail(userEmail, mailCC, fmt.Sprintf("Password reset link for %s", username), fmt.Sprintf("Open this link within %v minutes to reset the password for account %s: %s", h.config.TokenValidity, username, confirmLink)); err != nil {
		h.redisClient.Del(ctx, username)

		templData.ErrMessage = "Sorry, I was unable to send reset confirmation link by e-mail. Please try again later."
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	templData.Success = true
}
