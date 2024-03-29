package main

/*
1. pw reset request API endpoint
2. generate unique token -> store in redis
3. send mail with token + link
4. API endpoint for token and username validation (post nw pw)
5. set pw in IPA
6. Confirmation email
*/

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/vchrisr/go-freeipa/freeipa"
	"gopkg.in/gomail.v2"
)

type pwResetReqHandler struct {
	redisClient   *redis.Client
	mailClient    *gomail.Dialer
	ipaClient     *freeipa.Client
	config        appConfig
	BlockedGroups map[string]bool
}

func addDefaultHeaders(w *http.ResponseWriter) {
	l := *w
	l.Header().Set("X-Frame-Options", "SAMEORIGIN")
}

func NewPwResetReqHandler(config appConfig) *pwResetReqHandler {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	ipaClient, err := freeipa.Connect(config.IpaHost, tr, config.IpaUser, config.IpaPassword)
	if err != nil {
		log.Fatal(err)
	}

	mailTLSConfig := &tls.Config{InsecureSkipVerify: true}

	newHandler := pwResetReqHandler{
		redisClient: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%v:%v", config.RedisHost, config.RedisPort),
			Password: config.RedisPassword,
			DB:       config.RedisDB,
		}),
		mailClient: &gomail.Dialer{Host: config.EmailHost, Port: config.EmailPort, Username: config.EmailUser, Password: config.EmailPassword, TLSConfig: mailTLSConfig},
		ipaClient:  ipaClient,
		config:     config,
	}

	newHandler.BlockedGroups = make(map[string]bool)
	for _, v := range config.BlockedGroups {
		newHandler.BlockedGroups[v] = true
	}

	return &newHandler
}

func main() {
	config := LoadConfig()
	pwResetHandler := NewPwResetReqHandler(config)

	log.Println("Starting http server on port: ", config.AppPort)
	r := mux.NewRouter()
	r.Path("/requestreset").Methods(http.MethodPost).HandlerFunc(pwResetHandler.HandleResetRequest)
	r.Path("/confirmreset").Methods(http.MethodPost).HandlerFunc(pwResetHandler.HandleConfirmRequest)
	r.Path("/enterpw/{username}/{token}").Methods(http.MethodGet).HandlerFunc(pwResetHandler.PresentPwResetForm)
	r.Path("/").Methods(http.MethodGet).HandlerFunc(pwResetHandler.PresentResetRequestForm)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", config.AppPort), r))
}
