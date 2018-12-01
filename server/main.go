package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/Sirupsen/logrus"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

var staticFlag = flag.String("static", "", "Location of static files to serve")

func main() {
	logrus.Info("--- Starting server ---")
	if os.Getenv(oAuthClientID) == "" ||
		os.Getenv(oAuthClientSecret) == "" ||
		os.Getenv(oAuthCallback) == "" {
		logrus.Error("missing OAuth credentials")
		os.Exit(0)
	}

	router := gin.Default()

	router.GET("/env", env)
	router.GET("/api/v1/authorize", authorize)
	router.GET("/api/v1/svkprofile", svkProfile)

	// Liveliness probe
	router.GET("/healthz", func(c *gin.Context) {
		c.String(200, "OK")
	})

	flag.Parse()
	if *staticFlag != "" {
		logrus.Info("Static serving form: " + *staticFlag)
		router.Use(static.Serve("/", static.LocalFile(*staticFlag, false)))
		// Serve the index file when the browser requests a URl with HTML5 history
		router.NoRoute(func(c *gin.Context) {
			c.File(*staticFlag + "/index.html")
		})
	}

	router.Run() // listen and serve on 0.0.0.0:8080
}

func env(c *gin.Context) {
	vars := map[string]string{
		"OAUTH_CLIENT_ID": os.Getenv(oAuthClientID),
		"OAUTH_CALLBACK":  os.Getenv(oAuthCallback),
	}
	pairs := []string{}
	for k, v := range vars {
		pairs = append(pairs, k+":"+`"`+v+`"`)
	}
	js := strings.Join(pairs, ", ")
	c.String(200, "window.env = {%s}", js)
}

type authResponse struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

var oAuthClient = oauth2.Config{
	ClientID:     os.Getenv(oAuthClientID),
	ClientSecret: os.Getenv(oAuthClientSecret),
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://thewall.tibber.com/connect/authorize",
		TokenURL: "https://thewall.tibber.com/connect/token",
	},
	RedirectURL: os.Getenv(oAuthCallback),
	Scopes:      []string{"tibber_graph", "price", "consumption"},
}

func authorize(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	if c.Query("code") == "" {
		c.JSON(http.StatusBadRequest, errResponse{"query error"})
		return
	}

	oauthToken, err := oAuthClient.Exchange(oauth2.NoContext, c.Query("code"))
	if err != nil {
		logrus.Warn("auth error: ", err.Error())
		c.JSON(http.StatusUnauthorized, errResponse{"unauthorized"})
		return
	}

	if !oauthToken.Valid() {
		c.JSON(http.StatusUnauthorized, errResponse{"unauthorized"})
		return
	}

	c.JSON(200, authResponse{
		Token:   oauthToken.AccessToken,
		Expires: oauthToken.Expiry,
	})
}

func svkProfile(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")

	if c.Query("periodFrom") == "" ||
		c.Query("periodTo") == "" ||
		c.Query("networkAreaIdString") == "" {
		c.JSON(http.StatusBadRequest, errResponse{"query error"})
		return
	}

	url := `https://mimer.svk.se/` +
		`ConsumptionProfile/DownloadText` +
		`?groupByType=0` +
		`&periodFrom=` + c.Query("periodFrom") +
		`&periodTo=` + c.Query("periodTo") +
		`&networkAreaIdString=` + c.Query("networkAreaIdString")

	resp, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errResponse{"response error: " + err.Error()})
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errResponse{"body error: " + err.Error()})
	}
	c.String(200, string(bytes))
}

type errResponse struct {
	Error string `json:"error"`
}

// OS Env keys
const (
	oAuthClientID     = "OAUTH_CLIENT_ID"
	oAuthClientSecret = "OAUTH_CLIENT_SECRET"
	oAuthCallback     = "OAUTH_CALLBACK"
)