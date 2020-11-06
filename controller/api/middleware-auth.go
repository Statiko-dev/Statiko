/*
Copyright © 2020 Alessandro Segala (@ItalyPaleAle)

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

package api

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/shared/utils"
)

// Cache for the token RSA keys
var keyCache map[string]*rsa.PublicKey

// Minimum interval (in minutes) to download the JWKS keybag, and cache of last time it was checked
var jwksFetchInterval = 5 * time.Minute
var jwksLastFetch time.Time

// JWKS URL and JWT token issuer for Azure AD
const azureADJWKS = "https://login.microsoftonline.com/{tenant}/discovery/v2.0/keys"
const azureADIssuer = "https://login.microsoftonline.com/{tenant}/v2.0"

// JWKS URL and JWT token issuer for Auth0
const auth0JWKS = "https://{domain}/.well-known/jwks.json"
const auth0Issuer = "https://{domain}/"

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
func (s *APIServer) Auth(required bool) gin.HandlerFunc {
	// Check if an authentication provider is allowed; we can only support one at a time
	auth0Enabled := viper.GetBool("auth.auth0.enabled")
	azureADEnabled := viper.GetBool("auth.azureAD.enabled")
	if auth0Enabled && azureADEnabled {
		panic("only one external authentication provider can be enabled at any given time")
	}

	// Get the token key function
	var tokenKeyFunc jwt.Keyfunc
	var validateClaimFunc func(jwt.MapClaims) bool
	switch {
	case auth0Enabled:
		tokenKeyFunc = s.tokenKeyFuncGenerator("auth0")
		validateClaimFunc = s.validateClaimFuncGenerator("auth0")
	case azureADEnabled:
		tokenKeyFunc = s.tokenKeyFuncGenerator("azuread")
		validateClaimFunc = s.validateClaimFuncGenerator("azuread")
	}

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
			if viper.GetBool("auth.psk.enabled") && auth == viper.GetString("auth.psk.key") {
				// All good
				c.Set("authenticated", true)
				return
			}

			// Check if an authentication provider is allowed
			if auth0Enabled || azureADEnabled {
				token, err := jwt.Parse(auth, tokenKeyFunc)
				if err == nil {
					// Check claims and perform some extra checks
					if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid && validateClaimFunc(claims) {
						// All good
						c.Set("authenticated", true)
						return
					}
				}
			}
		}

		// If we're still here, authentication has failed
		// If the authentication was required, abort
		if required {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization header",
			})
		}
		return
	}
}

// Validate claims for a specific provider
func (s *APIServer) validateClaimFuncGenerator(provider string) func(jwt.MapClaims) bool {
	switch provider {
	case "auth0":
		return func(claims jwt.MapClaims) bool {
			// Perform some extra checks: iss, aud, then ensure exp and iat are present (they were validated already)
			audience := viper.GetString("auth.auth0.clientId")
			domain := viper.GetString("auth.auth0.domain")
			issuer := strings.Replace(auth0Issuer, "{domain}", domain, 1)
			if claims["iss"] == issuer && claims["aud"] == audience && claims["exp"] != "" && claims["iat"] != "" {
				return true
			}
			return false
		}
	case "azuread":
		return func(claims jwt.MapClaims) bool {
			// Perform some extra checks: iss, aud, then ensure exp and nbf are present (they were validated already)
			audience := viper.GetString("auth.azureAD.clientId")
			tenant := viper.GetString("auth.azureAD.tenantId")
			issuer := strings.Replace(azureADIssuer, "{tenant}", tenant, 1)
			if claims["iss"] == issuer && claims["aud"] == audience && claims["exp"] != "" && claims["nbf"] != "" {
				return true
			}
			return false
		}
	}
	return nil
}

// Function used to return the key used to sign the tokens
func (s *APIServer) tokenKeyFuncGenerator(provider string) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		// Azure AD tokens are signed with RS256 method
		if token.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
		}

		// Get the signing key
		key, err := s.getTokenSigningKey(token.Header["kid"].(string), provider)
		if err != nil {
			s.logger.Println("[Error] Error while requesting token signing key:", err)
			return nil, err
		}
		if key == nil {
			s.logger.Println("[Error] Key not found in the key store")
			return nil, errors.New("key not found")
		}
		return key, nil
	}
}

// Get the token signing keys
func (s *APIServer) getTokenSigningKey(kid string, provider string) (key *rsa.PublicKey, err error) {
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
		s.logger.Println("Key not found in JWKS cache, but last fetch was too recent")
		// Just return a "not found"
		key = nil
		return
	}

	// Need to request the key
	var issuer, url string
	switch provider {
	case "auth0":
		domain := viper.GetString("auth.auth0.domain")
		url = strings.Replace(auth0JWKS, "{domain}", domain, 1)
		// Issuer is not present in the JWKS from Auth0
	case "azuread":
		tenant := viper.GetString("auth.azureAD.tenantId")
		issuer = strings.Replace(azureADIssuer, "{tenant}", tenant, 1)
		url = strings.Replace(azureADJWKS, "{tenant}", tenant, 1)
	}
	s.logger.Println("Fetching JWKS from " + url)
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
	jwksLastFetch = time.Now()

	// Iterate through the keys and add them to cache
	if len(jwks.Keys) < 1 {
		err = errors.New("empty JWKS keybag")
		return
	}
	for _, el := range jwks.Keys {
		// Skip invalid keys
		if el.Kty != "RSA" || el.Use != "sig" || (issuer != "" && el.Issuer != issuer) || el.N == "" || el.E == "" || el.Kid == "" {
			continue
		}
		// Parse the key
		k, err := utils.ParseRSAPublicKey(el.N, el.E)
		if err != nil {
			return nil, err
		}
		// Add to cache
		s.logger.Println("Received JWKS key " + el.Kid)
		keyCache[el.Kid] = k
	}

	// Return the key from the cache, if any
	key, found = keyCache[kid]
	if !found {
		key = nil
	}
	return
}
