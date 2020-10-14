package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hyperjumptech/hansip/internal/config"
	"github.com/hyperjumptech/hansip/internal/constants"
	"github.com/hyperjumptech/hansip/internal/hansipcontext"
	"github.com/hyperjumptech/hansip/pkg/helper"
	log "github.com/sirupsen/logrus"
)

var (
	jwtMiddLog = log.WithField("go", "JwtMiddleware")

	// TokenFactory instance used within this middleware
	TokenFactory helper.TokenFactory

	whiteListUrls = []string{
		"/docs/**/*",
		"/api/v1/auth/authenticate",
		"/api/v1/auth/2fa",
		"/api/v1/recovery/**/*",
		"/api/v1/management/user/activate",
	}

	adminRole = config.Get("user.role.admin")
	userRole  = config.Get("user.role.user")

	// ACLs is Access Control List
	ACLs = []*ACL{
		&ACL{PathPattern: "/docs/**/*", AllowedAudiences: []string{}, Method: "GET"},
		&ACL{PathPattern: "/health", AllowedAudiences: []string{}, Method: "GET"},

		&ACL{PathPattern: "/api/v1/auth/authenticate", AllowedAudiences: []string{}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/auth/refresh", AllowedAudiences: []string{userRole, adminRole}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/auth/2fa", AllowedAudiences: []string{}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/auth/2fatest", AllowedAudiences: []string{}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/auth/authenticate2fa", AllowedAudiences: []string{}, Method: "POST"},

		&ACL{PathPattern: "/api/v1/recovery/**/*", AllowedAudiences: []string{}, Method: "POST"},

		&ACL{PathPattern: "/api/v1/management/user/whoami", AllowedAudiences: []string{userRole, adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/user/activate", AllowedAudiences: []string{}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/management/users", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/user", AllowedAudiences: []string{adminRole}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/management/user/*/passwd", AllowedAudiences: []string{adminRole, userRole}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/management/user/activate", AllowedAudiences: []string{adminRole}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/management/user/*", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/user/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/user/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/user/*/roles", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/user/*/roles", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/user/*/roles", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/user/*/all-roles", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/user/*/role/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/user/*/role/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/user/*/groups", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/user/*/groups", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/user/*/groups", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/user/*/group/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/user/*/group/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/user/2FAQR", AllowedAudiences: []string{adminRole, userRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/user/activate2FA", AllowedAudiences: []string{adminRole, userRole}, Method: "POST"},

		&ACL{PathPattern: "/api/v1/management/groups", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/group", AllowedAudiences: []string{adminRole}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/management/group/*", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/group/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/group/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/group/*/users", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/group/*/users", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/group/*/users", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/group/*/user/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/group/*/user/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/group/*/roles", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/group/*/roles", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/group/*/roles", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/group/*/role/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/group/*/role/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},

		&ACL{PathPattern: "/api/v1/management/roles", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/role", AllowedAudiences: []string{adminRole}, Method: "POST"},
		&ACL{PathPattern: "/api/v1/management/role/*", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/role/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/role/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/role/*/users", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/role/*/users", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/role/*/users", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/role/*/user/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/role/*/user/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/role/*/groups", AllowedAudiences: []string{adminRole}, Method: "GET"},
		&ACL{PathPattern: "/api/v1/management/role/*/groups", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/role/*/groups", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
		&ACL{PathPattern: "/api/v1/management/role/*/group/*", AllowedAudiences: []string{adminRole}, Method: "PUT"},
		&ACL{PathPattern: "/api/v1/management/role/*/group/*", AllowedAudiences: []string{adminRole}, Method: "DELETE"},
	}
)

// ACL represent an access control record
type ACL struct {
	PathPattern      string
	Method           string
	AllowedAudiences []string
}

// JwtMiddleware handle authorization check for accessed endpoint by inspecting the Authorization header and look for JWT token.
func JwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fLog := jwtMiddLog.WithField("path", r.URL.Path).WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("func", "jwtMiddLog").WithField("method", r.Method)
		fLog.Tracef("Checking JWT")
		for _, acl := range ACLs {
			match, err := helper.Match(acl.PathPattern, r.URL.Path)
			if err != nil {
				panic(err)
			}
			if match {
				fLog.Tracef("Found ACL %s. Checking method", acl.PathPattern)
				// First we check the method agains ACL
				if strings.ToUpper(r.Method) == acl.Method {
					fLog.Tracef("Found ACL %s for method %s", acl.PathPattern, acl.Method)
					// If audience empty, its whitelisted, proceed.
					if len(acl.AllowedAudiences) == 0 {
						fLog.Tracef("Allowing %s for there are no allowed audience rule", r.URL.Path)
						next.ServeHTTP(w, r)
						return
					}
					// If it need validation, Check the Authorization header
					authHeader := r.Header.Get("Authorization")
					if len(authHeader) == 0 {
						fLog.Warnf("Missing 'Authorization' header")
						helper.WriteHTTPResponse(r.Context(), w, http.StatusUnauthorized, fmt.Sprintf("You are not authorized to access this end point %s", r.URL.Path), nil, nil)
						return
					}
					meth := strings.ToLower(strings.TrimSpace(authHeader[:6]))
					if meth != "bearer" {
						fLog.Warnf("Authorization method missing `bearer`")
						helper.WriteHTTPResponse(r.Context(), w, http.StatusUnauthorized, fmt.Sprintf("Authorization method %s not supported", meth), nil, nil)
						return
					}
					// Get the token, validate and parse it.
					tok := strings.TrimSpace(authHeader[7:])
					issuer, subject, audience, _, _, _, additional, err := TokenFactory.ReadToken(tok)
					if err != nil {
						fLog.Warnf(fmt.Sprintf("Token validation error. got %s", err.Error()))
						helper.WriteHTTPResponse(r.Context(), w, http.StatusForbidden, fmt.Sprintf("Forbidden. Got %s ", err.Error()), nil, nil)
						return
					}
					// Makesure the issuer is the same
					if issuer != config.Get("token.issuer") {
						fLog.Warnf(fmt.Sprintf("Invalid issuer. Expect %s but %s. got %s", config.Get("token.issuer"), issuer, err.Error()))
						helper.WriteHTTPResponse(r.Context(), w, http.StatusForbidden, fmt.Sprintf("Forbidden. Not accepting token from issuer %s ", issuer), nil, nil)
						return
					}
					allowed := false
					for _, allowedAud := range acl.AllowedAudiences {
						if helper.StringArrayContainString(audience, allowedAud) {
							allowed = true
							break
						}
					}
					if allowed {
						fLog.Tracef("Allowing token audiences %v, verified by allowed %v", audience, acl.AllowedAudiences)
						hansipContext := &hansipcontext.AuthenticationContext{
							Token:     tok,
							Subject:   subject,
							Audience:  audience,
							TokenType: additional["type"].(string),
						}
						tokenCtx := context.WithValue(r.Context(), constants.HansipAuthentication, hansipContext)
						next.ServeHTTP(w, r.WithContext(tokenCtx))
						return
					}
				}
			}
		}
		fLog.Warnf("Forbidden")
		helper.WriteHTTPResponse(r.Context(), w, http.StatusForbidden, fmt.Sprintf("You are authenticated but not allowed to access this end point %s", r.URL.Path), nil, nil)
		return
	})
}
