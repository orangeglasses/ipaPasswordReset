package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sethvargo/go-password/password"
	"github.com/vchrisr/go-freeipa/freeipa"
	"golang.org/x/net/context"
)

func (h pwResetReqHandler) HandleConfirmRequest(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	reqUsername := r.FormValue("username")
	reqToken := r.FormValue("token")

	reqPassword := ""
	tmplFile := "ackReset.tmpl"
	isSvcAccount := h.userInSvcAccountPrefixes(reqUsername)
	expMonths := h.config.UserPwValidityMonths
	if isSvcAccount {
		fmt.Println("Issvcaccount")
		tmplFile = "ackSvcAccReset.tmpl"
		var err error
		reqPassword, err = password.Generate(h.config.SvcAccPasswordLength, int(h.config.SvcAccPasswordLength/4), 0, false, false)
		if err != nil {
			fmt.Println(err)
		}
		expMonths = h.config.SvcAccPwValidityMonths
	} else {
		reqPassword = r.FormValue("password")
	}

	tmpl := template.Must(template.ParseFiles(tmplFile))

	templData := struct {
		Success    bool
		Username   string
		Password   string
		ErrMessage string
		AppName    string
	}{
		Success:    false,
		Username:   reqUsername,
		ErrMessage: "General error",
		AppName:    h.config.AppName,
		Password:   reqPassword,
	}

	defer func() {
		r.Body.Close()
		tmpl.Execute(w, templData)
	}()

	addDefaultHeaders(&w)
	ctx := context.Background()

	//get token and Check token matches username
	token, err := h.redisClient.Get(ctx, reqUsername).Result()
	if err != nil || token != reqToken {
		templData.ErrMessage = "Invalid or expired username or token"
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if len(reqPassword) < h.config.MinPasswordLength {
		templData.ErrMessage = "Password does not meet minimum length requirement"
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
	userEmail, mailCC := h.getUserMail(ipaUserResult.Result)

	//Check again if user is not in blocked group. This can only happen when Redis is compromised.
	if h.userInBlockedGroup(ipaUserResult.Result.MemberofGroup) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//check if account is locked. Action is configurable
	if *ipaUserResult.Result.Nsaccountlock {
		if !h.config.IpaEnableAccountOnReset {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		_, err = h.ipaClient.UserEnable(&freeipa.UserEnableArgs{}, &freeipa.UserEnableOptionalArgs{UID: &reqUsername})
		if err != nil {
			log.Printf("enable failed: %s\n", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	//Reset PW
	_, err = h.ipaClient.Passwd(&freeipa.PasswdArgs{Principal: reqUsername, Password: reqPassword}, &freeipa.PasswdOptionalArgs{})
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Printf("Password for user %s successfully reset\n", reqUsername)

	//Set expiration date
	expDate := time.Now().In(time.UTC).Truncate(time.Second).AddDate(0, expMonths, 0)

	_, err = h.ipaClient.UserMod(&freeipa.UserModArgs{}, &freeipa.UserModOptionalArgs{UID: &reqUsername, Krbpasswordexpiration: &expDate})
	if err != nil && !strings.HasPrefix(err.Error(), "unexpected value for field Krbpasswordexpiration") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Printf("Expiration date for user %s successfully set to %v\n", reqUsername, expDate)

	//Send confirmation mail
	h.sendMail(userEmail, mailCC, "Password Reset Completed", fmt.Sprintf("The password for account %s was reset. If you did not request a password reset please contact your admin asap!", reqUsername))

	//Delete token from Redis
	if err := h.redisClient.Del(ctx, reqUsername).Err(); err != nil {
		log.Println("Error deleting token from redis: ", err)
	}

	templData.Success = true
}
