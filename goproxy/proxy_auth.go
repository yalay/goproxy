package main
import (
	"encoding/base64"
	"net/http"
	"strings"
)

func BasicAuth(w http.ResponseWriter, r *http.Request, username string, password string) bool {

	pheader := r.Header["Proxy-Authorization"]

	if pheader==nil || len(pheader)==0 {

		return false
	}

	auth := strings.SplitN(pheader[0], " ", 2)

	if len(auth) != 2 || auth[0] != "Basic" {

		return false
	}

	payload, _ := base64.StdEncoding.DecodeString(auth[1])

	pair := strings.SplitN(string(payload), ":", 2)

	if len(pair) != 2  {

		return false
	}

	if pair[0]!=username || pair[1]!=password {

		return false
	}

	return true

}
