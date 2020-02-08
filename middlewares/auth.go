/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, version 3 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package middlewares

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"

	"smplatform/appconfig"
	"smplatform/utils"
)

// Cache for the token RSA keys
var keyCache map[string]*rsa.PublicKey

// Minimum interval (in minutes) to download the JWKS keybag, and cache of last time it was checked
var jwksFetchInterval = 5 * time.Minute
var jwksLastFetch time.Time

// JWKS URL for Azure AD
const azureADJWKS = "https://login.microsoftonline.com/{tenant}/discovery/v2.0/keys"

// JWT Token issuer for Azure AD
const azureADIssuer = "https://login.microsoftonline.com/{tenant}/v2.0"

// Key response from the JWKS endpoint
type jwksKey struct {
	Kty    string `json:"kty"`
	Use    string `json:"use"`
	Kid    string `json:"kid"`
	N      string `json:"n"`
	E      string `json:"e"`
	Issuer string `json:"issuer"`
}

// Auth middleware that checks the Authorization header in the request
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check the Authorization header, and stop invalid requests
		auth := c.GetHeader("Authorization")
		// Remove spaces around token
		auth = strings.TrimSpace(auth)

		// If the token begins with "Bearer ", remove that (we make it optional)
		if strings.HasPrefix(auth, "Bearer ") {
			auth = auth[7:]
		}

		if len(auth) != 0 {
			// If pre-shared keys are allowed, check if there's a match
			if appconfig.Config.GetBool("auth.psk.enabled") && auth == appconfig.Config.GetString("auth.psk.key") {
				// All good
				return
			}

			// Check if Azure AD authentication is allowed
			if appconfig.Config.GetBool("auth.azureAD.enabled") {
				token, err := jwt.Parse(auth, func(token *jwt.Token) (interface{}, error) {
					// Azure AD tokens are signed with RS256 method
					if token.Method.Alg() != "RS256" {
						return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
					}

					// Get the signing key
					key, err := getTokenSigningKey(token.Header["kid"].(string))
					if err != nil {
						logger.Println("[Error] Error while requesting token signing key:", err)
						return nil, err
					}
					return key, nil
				})
				if err == nil {
					// Check claims
					if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
						// Perform some extra checks: iss, aud, then ensure exp and nbf are present (they were validated already)
						audience := appconfig.Config.GetString("auth.azureAD.clientId")
						tenant := appconfig.Config.GetString("auth.azureAD.tenantId")
						issuer := strings.Replace(azureADIssuer, "{tenant}", tenant, 1)
						if claims["iss"] == issuer && claims["aud"] == audience && claims["exp"] != "" && claims["nbf"] != "" {
							// All good
							return
						}
					}
				}
			}
		}

		// If we're still here, authentication has failed
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid Authorization header",
		})
		return
	}
}

// Get the token signing keys
func getTokenSigningKey(kid string) (key *rsa.PublicKey, err error) {
	// Check if we have the key in memory
	if keyCache == nil {
		keyCache = make(map[string]*rsa.PublicKey)
	}
	var found bool
	key, found = keyCache[kid]
	if found && key != nil {
		return
	}

	// Do not request keys again if it's been less than N minutes
	if time.Now().Before(jwksLastFetch.Add(jwksFetchInterval)) {
		logger.Println("Key not found in JWKS cache, but last fetch was too recent")
		// Just return a "not found"
		key = nil
		return
	}

	// Need to request the key
	tenant := appconfig.Config.GetString("auth.azureAD.tenantId")
	issuer := strings.Replace(azureADIssuer, "{tenant}", tenant, 1)
	url := strings.Replace(azureADJWKS, "{tenant}", tenant, 1)
	logger.Println("Fetching JWKS from " + url)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	jwks := struct {
		Keys []jwksKey `json:"keys"`
	}{
		Keys: make([]jwksKey, 0),
	}
	err = utils.RequestJSON(client, url, &jwks)
	if err != nil {
		return
	}

	// Iterate through the keys and add them to cache
	if len(jwks.Keys) < 1 {
		err = errors.New("empty JWKS keybag")
		return
	}
	for _, el := range jwks.Keys {
		// Skip invalid keys
		if el.Kty != "RSA" || el.Use != "sig" || el.Issuer != issuer || el.N == "" || el.E == "" || el.Kid == "" {
			continue
		}
		// Parse the key
		k, err := utils.ParseRSAPublicKey(el.N, el.E)
		if err != nil {
			return nil, err
		}
		// Add to cache
		logger.Println("Received JWKS key " + el.Kid)
		keyCache[el.Kid] = k
	}

	// Return the key from the cache, if any
	key, found = keyCache[kid]
	if !found {
		key = nil
	}
	return
}
