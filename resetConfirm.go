package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/vchrisr/go-freeipa/freeipa"
	"golang.org/x/net/context"
)

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

	//get token and Check token matches username
	token, err := h.redisClient.Get(ctx, reqUsername).Result()
	if err != nil || token != reqToken {
		templData.ErrMessage = "Invalid or expired username or token"
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

	//Check again if user is not in blocked group. This can only happen when Redis is compromised.
	if h.userInBlockedGroup(*ipaUserResult.Result.MemberofGroup) {
		return
	}

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
	h.sendMail(userEmail, "Password Reset Completed", "Your password was reset. If you did not request a password reset please contact your admin asap!")

	//Delete token from Redis
	if err := h.redisClient.Del(ctx, reqUsername).Err(); err != nil {
		log.Println("Error deleting token from redis: ", err)
	}

	templData.Success = true
}
