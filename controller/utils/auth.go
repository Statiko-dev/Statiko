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

package utils

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/shared/utils"
	sharedutils "github.com/statiko-dev/statiko/shared/utils"
)

// Logger for this file
var authLogger = log.New(buildinfo.LogDestination, "auth: ", log.Ldate|log.Ltime|log.LUTC)

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

// Authentication methods that are enabled
var authAuth0Enabled = false
var authAzureADEnabled = false
var authPSKEnabled = false

// Callback that provides the key for validating a JWT
var tokenKeyFunc jwt.Keyfunc

// Function that validates the claims inside a JWT
var validateClaimFunc func(jwt.MapClaims) bool

// Flag will be set to true after callbacks are loaded
var authCallbacksLoaded = false

// If an authentication provider such as Auth0 or AzureAD are enabled, returns the functions to validate the JWT token and claims
func authLoadCallbacks() {
	// If we have already invoked this function, return
	if authCallbacksLoaded {
		return
	}

	// Get these flags in variables for the package so we don't have to invoke viper on every request
	authAuth0Enabled = viper.GetBool("auth.auth0.enabled")
	authAzureADEnabled = viper.GetBool("auth.azureAD.enabled")
	authPSKEnabled = viper.GetBool("auth.psk.enabled")

	// Check if an external authentication provider is allowed; we can only support one at a time
	if authAuth0Enabled && authAzureADEnabled {
		authLogger.Fatal("only one external authentication provider can be enabled at any given time")
	}

	// At least one external authentication provider or PSK must be enabled
	if !(authAuth0Enabled || authAzureADEnabled || authPSKEnabled) {
		authLogger.Fatal("at least one authentication provider must be enabled")
	}

	// Get the callback functions
	switch {
	case authAuth0Enabled:
		tokenKeyFunc = authTokenKeyFuncGenerator("auth0")
		validateClaimFunc = authValidateClaimFuncGenerator("auth0")
	case authAzureADEnabled:
		tokenKeyFunc = authTokenKeyFuncGenerator("azuread")
		validateClaimFunc = authValidateClaimFuncGenerator("azuread")
	}

	// All done
	authCallbacksLoaded = true
}

// Validates an authentication token (PSK or JWT)
func authValidate(auth string) error {
	// Remove spaces around the auth token
	// If the token begins with "Bearer ", remove that too (we make it optional)
	auth = strings.TrimPrefix(strings.TrimSpace(auth), "Bearer ")
	if auth == "" {
		return errors.New("invalid authorization")
	}

	// If pre-shared keys are allowed, check if there's a match
	if authPSKEnabled && auth == viper.GetString("auth.psk.key") {
		// All good
		return nil
	}

	// Check if an authentication provider is allowed
	if authAuth0Enabled || authAzureADEnabled {
		token, err := jwt.Parse(auth, tokenKeyFunc)
		if err != nil {
			return err
		}

		// Check claims and perform some extra checks
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid || !validateClaimFunc(claims) {
			return errors.New("invalid authorization")
		}

		// All good
		return nil
	}

	// We should never reach this, but…
	return errors.New("invalid authorization")
}

// AuthGinMiddleware returns a gin middleware that checks the Authorization header in the request
// If required is false, then requests whose Authentication header is invalid can still go through, but are marked as un-authenticated
func AuthGinMiddleware(required bool) gin.HandlerFunc {
	// Load the auth callbacks if not already done
	// Also, this ensures that we have at most one authentication provider (in addition to the optional PSK)
	authLoadCallbacks()

	// Return a middleware for gin
	return func(c *gin.Context) {
		// Use the Authorization header for authenticating requests
		err := authValidate(c.GetHeader("Authorization"))
		if err != nil {
			// If we're here, authentication has failed
			// If the authentication was required, abort the request (so no more middleware are processed)
			if required {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid Authorization header",
				})
			}
			// In every case, return
			return
		}

		// The request is authenticated, so add that to the request's context
		c.Set("authenticated", true)
		return
	}
}

// AuthGRPCUnaryInterceptor returns an interceptor for unary ("simple") gRPC requests that checks the authorization field in the metadata
// The "excludeMethods" slice contains an optional list of full method names that don't require authentication
func AuthGRPCUnaryInterceptor(excludeMethods []string) grpc.UnaryServerInterceptor {
	// Load the auth callbacks if not already done
	// Also, this ensures that we have at most one authentication provider (in addition to the optional PSK)
	authLoadCallbacks()

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		// Check if this method is always allowed, even without authorization
		if len(excludeMethods) > 0 && utils.StringInSlice(excludeMethods, info.FullMethod) {
			// Skip checking authorization and just continue the execution
			return handler(ctx, req)
		}

		// Check if the call is authorized
		err = authGRPCCheckMetadata(ctx)
		if err != nil {
			return
		}

		// Call is authorized, so continue the execution
		return handler(ctx, req)
	}
}

// AuthGRPCStreamInterceptor is an interceptor for stream gRPC requests that checks the authorization field in the metadata
func AuthGRPCStreamInterceptor(srv interface{}, srvStream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	// Check if the call is authorized
	err = authGRPCCheckMetadata(srvStream.Context())
	if err != nil {
		return
	}

	// Call is authorized, so continue the execution
	return handler(srv, srvStream)
}

// Used by the gRPC auth interceptors, this checks the authorization metadata
func authGRPCCheckMetadata(ctx context.Context) error {
	// Ensure we have an authorization metadata
	// Note that the keys in the metadata object are always lowercased
	m, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return grpc.Errorf(codes.Unauthenticated, "missing metadata")
	}
	if len(m["authorization"]) != 1 {
		return grpc.Errorf(codes.Unauthenticated, "invalid authorization")
	}

	// Validate the authorization field
	err := authValidate(m["authorization"][0])
	if err != nil {
		return grpc.Errorf(codes.Unauthenticated, err.Error())
	}
	return nil
}

// Validate claims for a specific provider
func authValidateClaimFuncGenerator(provider string) func(jwt.MapClaims) bool {
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
func authTokenKeyFuncGenerator(provider string) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		// Azure AD tokens are signed with RS256 method
		if token.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
		}

		// Get the signing key
		key, err := authGetTokenSigningKey(token.Header["kid"].(string), provider)
		if err != nil {
			authLogger.Println("[Error] Error while requesting token signing key:", err)
			return nil, err
		}
		if key == nil {
			authLogger.Println("[Error] Key not found in the key store")
			return nil, errors.New("key not found")
		}
		return key, nil
	}
}

// Key response from the JWKS endpoint
type authJwksKey struct {
	Kty    string `json:"kty"`
	Use    string `json:"use"`
	Kid    string `json:"kid"`
	N      string `json:"n"`
	E      string `json:"e"`
	Issuer string `json:"issuer"`
}

// Get the token signing keys
func authGetTokenSigningKey(kid string, provider string) (key *rsa.PublicKey, err error) {
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
		authLogger.Println("Key not found in JWKS cache, but last fetch was too recent")
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
	authLogger.Println("Fetching JWKS from " + url)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	jwks := struct {
		Keys []authJwksKey `json:"keys"`
	}{
		Keys: make([]authJwksKey, 0),
	}
	err = sharedutils.RequestJSON(client, url, &jwks)
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
		k, err := ParseRSAPublicKey(el.N, el.E)
		if err != nil {
			return nil, err
		}
		// Add to cache
		authLogger.Println("Received JWKS key " + el.Kid)
		keyCache[el.Kid] = k
	}

	// Return the key from the cache, if any
	key, found = keyCache[kid]
	if !found {
		key = nil
	}
	return
}
