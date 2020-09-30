package mgmnt

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	"github.com/hyperjumptech/hansip/internal/config"
	"github.com/hyperjumptech/hansip/internal/constants"
	"github.com/hyperjumptech/hansip/internal/hansipcontext"
	"github.com/hyperjumptech/hansip/internal/mailer"
	"github.com/hyperjumptech/hansip/pkg/helper"
	"github.com/hyperjumptech/hansip/pkg/totp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var (
	userMgmtLogger = log.WithField("go", "UserManagement")
)

// Show2FAQrCode shows 2FA QR code. It returns a PNG image bytes.
func Show2FAQrCode(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "Show2FAQrCode").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	authCtx := r.Context().Value(constants.HansipAuthentication).(*hansipcontext.AuthenticationContext)
	user, err := UserRepo.GetUserByEmail(r.Context(), authCtx.Subject)
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByEmail got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, fmt.Sprintf("subject not found : %s. got %s", authCtx.Subject, err.Error()))
		return
	}

	user.UserTotpSecretKey = totp.MakeRandomTotpKey()
	err = UserRepo.SaveOrUpdate(r.Context(), user)
	if err != nil {
		fLog.Errorf("UserRepo.SaveOrUpdate got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	fLog.Warnf("New TOTP secret is created for %s", user.Email)

	codes, err := UserRepo.RecreateTOTPRecoveryCodes(r.Context(), user)
	if err != nil {
		fLog.Errorf("UserRepo.RecreateTOTPRecoveryCodes got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	fLog.Warnf("Created %d recovery codes for %s", len(codes), user.Email)

	secretstr := strings.TrimRight(base32.StdEncoding.EncodeToString([]byte(user.UserTotpSecretKey)), "=")

	png, err := totp.MakeTotpQrImage(secretstr, fmt.Sprintf("AAA:%s", user.Email))
	if err != nil {
		fLog.Errorf("totp.MakeTotpQrImage got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	w.Header().Add("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	w.Write(png)
}

// SimpleUser hold data model of user. showing important attributes only.
type SimpleUser struct {
	RecID     string `json:"rec_id"`
	Email     string `json:"email"`
	Enabled   bool   `json:"enabled"`
	Suspended bool   `json:"suspended"`
}

// ListAllUsers serving listing all user request
func ListAllUsers(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, "Mohomaaf ...dsb", nil, nil)
		}
	}()

	fLog := userMgmtLogger.WithField("func", "ListAllUsers").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	fLog.Trace("Listing Users")
	pageRequest, err := helper.NewPageRequestFromRequest(r)
	if err != nil {
		fLog.Errorf("helper.NewPageRequestFromRequest got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	users, page, err := UserRepo.ListUser(r.Context(), pageRequest)
	if err != nil {
		fLog.Errorf("UserRepo.ListUser got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	susers := make([]*SimpleUser, len(users))
	for i, v := range users {
		susers[i] = &SimpleUser{
			RecID:     v.RecID,
			Email:     v.Email,
			Enabled:   v.Enabled,
			Suspended: v.Suspended,
		}
	}
	ret := make(map[string]interface{})
	ret["users"] = susers
	ret["page"] = page
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "List of all user paginated", nil, ret)
}

// CreateNewUserRequest hold the data model for requesting to create new user.
type CreateNewUserRequest struct {
	Email      string `json:"email"`
	Passphrase string `json:"passphrase"`
}

// CreateNewUserResponse hold the data model for responding CreateNewUser request
type CreateNewUserResponse struct {
	RecordID    string    `json:"rec_id"`
	Email       string    `json:"email"`
	Enabled     bool      `json:"enabled"`
	Suspended   bool      `json:"suspended"`
	LastSeen    time.Time `json:"last_seen"`
	LastLogin   time.Time `json:"last_login"`
	TotpEnabled bool      `json:"2fa_enabled"`
}

// CreateNewUser handles request to create new user
func CreateNewUser(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "CreateNewUser").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	fLog.Trace("Creating new user")
	req := &CreateNewUserRequest{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fLog.Errorf("ioutil.ReadAll got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	err = json.Unmarshal(body, req)
	if err != nil {
		fLog.Errorf("json.Unmarshal got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	user, err := UserRepo.CreateUserRecord(r.Context(), req.Email, req.Passphrase)
	if err != nil {
		fLog.Errorf("UserRepo.CreateUserRecord got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	resp := &CreateNewUserResponse{
		RecordID:    user.RecID,
		Email:       user.Email,
		Enabled:     user.Enabled,
		Suspended:   user.Suspended,
		LastSeen:    user.LastSeen,
		LastLogin:   user.LastLogin,
		TotpEnabled: user.Enable2FactorAuth,
	}
	fLog.Warnf("Sending email")
	mailer.Send(r.Context(), &mailer.Email{
		From:     config.Get("mailer.from"),
		FromName: config.Get("mailer.from.name"),
		To:       []string{user.Email},
		Cc:       nil,
		Bcc:      nil,
		Template: "EMAIL_VERIFY",
		Data:     user,
	})

	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "Success creating user", nil, resp)
	return
}

// ChangePassphraseRequest stores change password request
type ChangePassphraseRequest struct {
	OldPassphrase string `json:"old_passphrase"`
	NewPassphrase string `json:"new_passphrase"`
}

// ChangePassphrase handles the change password request
func ChangePassphrase(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "ChangePassphrase").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/passwd", r.URL.Path)
	if err != nil {
		panic(err)
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fLog.Errorf("ioutil.ReadAll got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	c := &ChangePassphraseRequest{}
	err = json.Unmarshal(body, c)
	if err != nil {
		fLog.Errorf("json.Unmarshal got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, "Malformed json body", nil, nil)
		return
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassphrase), []byte(c.OldPassphrase))
	if err != nil {
		fLog.Errorf("bcrypt.CompareHashAndPassword got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotAcceptable, err.Error(), nil, nil)
		return
	}
	newHashed, err := bcrypt.GenerateFromPassword([]byte(c.NewPassphrase), 14)
	if err != nil {
		fLog.Errorf("bcrypt.GenerateFromPassword got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	user.HashedPassphrase = string(newHashed)
	err = UserRepo.SaveOrUpdate(r.Context(), user)
	if err != nil {
		fLog.Errorf("UserRepo.SaveOrUpdate got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "Password changed", nil, nil)
}

// ActivateUserRequest hold request data for activating user
type ActivateUserRequest struct {
	Email           string `json:"email"`
	ActivationToken string `json:"activation_token"`
}

// WhoAmIResponse holds the response structure for WhoAmI request
type WhoAmIResponse struct {
	RecordID  string          `json:"rec_id"`
	Email     string          `json:"email"`
	Enabled   bool            `json:"enabled"`
	Suspended bool            `json:"suspended"`
	Roles     []*RoleSummary  `json:"roles"`
	Groups    []*GroupSummary `json:"groups"`
}

// RoleSummary hold role information summay
type RoleSummary struct {
	RecordID string `json:"rec_id"`
	RoleName string `json:"role_name"`
}

// GroupSummary hold group information summay
type GroupSummary struct {
	RecordID  string         `json:"rec_id"`
	GroupName string         `json:"group_name"`
	Roles     []*RoleSummary `json:"roles"`
}

// Activate2FARequest hold request structure for activating the 2FA request
type Activate2FARequest struct {
	Token string `json:"2FA_token"`
}

// Activate2FAResponse hold response structure for activating the 2FA request
type Activate2FAResponse struct {
	Codes []string `json:"2FA_recovery_codes"`
}

// Activate2FA handle 2FA activation request
func Activate2FA(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "Activate2FA").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	authCtx := r.Context().Value(constants.HansipAuthentication).(*hansipcontext.AuthenticationContext)
	user, err := UserRepo.GetUserByEmail(r.Context(), authCtx.Subject)
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByEmail got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, fmt.Sprintf("subject not found : %s. got %s", authCtx.Subject, err.Error()))
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fLog.Errorf("ioutil.ReadAll got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	c := &Activate2FARequest{}
	err = json.Unmarshal(body, c)
	if err != nil {
		fLog.Errorf("json.Unmarshal got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, "Malformed json body", nil, nil)
		return
	}
	otp, err := totp.GenerateTotpWithDrift(user.UserTotpSecretKey, time.Now().UTC(), 30, 6)
	if err != nil {
		fLog.Errorf("totp.GenerateTotpWithDrift got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	if c.Token != otp {
		fLog.Errorf("Invalid OTP token for %s", user.Email)
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, "Invalid OTP")
		return
	}
	codes, err := UserRepo.GetTOTPRecoveryCodes(r.Context(), user)
	if err != nil {
		fLog.Errorf("UserRepo.GetTOTPRecoveryCodes got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	resp := Activate2FAResponse{
		Codes: codes,
	}
	user.Enable2FactorAuth = true
	err = UserRepo.SaveOrUpdate(r.Context(), user)
	if err != nil {
		fLog.Errorf("UserRepo.SaveOrUpdate got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "2FA Activated", nil, resp)
}

// WhoAmI handles who am I inquiry request
func WhoAmI(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "WhoAmI").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	authCtx := r.Context().Value(constants.HansipAuthentication).(*hansipcontext.AuthenticationContext)
	user, err := UserRepo.GetUserByEmail(r.Context(), authCtx.Subject)
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByEmail got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, fmt.Sprintf("subject not found : %s. got %s", authCtx.Subject, err.Error()))
		return
	}
	whoami := &WhoAmIResponse{
		RecordID:  user.RecID,
		Email:     user.Email,
		Enabled:   user.Enabled,
		Suspended: user.Suspended,
		Roles:     make([]*RoleSummary, 0),
		Groups:    make([]*GroupSummary, 0),
	}
	roles, _, err := UserRoleRepo.ListUserRoleByUser(r.Context(), user, &helper.PageRequest{
		No:       1,
		PageSize: 100,
		OrderBy:  "ROLE_NAME",
		Sort:     "ASC",
	})
	if err != nil {
		fLog.Errorf("UserRoleRepo.ListUserRoleByUser got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, fmt.Sprintf("subject not found : %s. got %s", authCtx.Subject, err.Error()))
		return
	}
	for _, r := range roles {
		whoami.Roles = append(whoami.Roles, &RoleSummary{
			RecordID: r.RecID,
			RoleName: r.RoleName,
		})
	}

	groups, _, err := UserGroupRepo.ListUserGroupByUser(r.Context(), user, &helper.PageRequest{
		No:       1,
		PageSize: 100,
		OrderBy:  "GROUP_NAME",
		Sort:     "ASC",
	})
	if err != nil {
		fLog.Errorf("UserGroupRepo.ListUserGroupByUser got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, fmt.Sprintf("subject not found : %s. got %s", authCtx.Subject, err.Error()))
		return
	}
	for _, g := range groups {
		groupSummary := &GroupSummary{
			RecordID:  g.RecID,
			GroupName: g.GroupName,
			Roles:     make([]*RoleSummary, 0),
		}
		groupRole, _, err := GroupRoleRepo.ListGroupRoleByGroup(r.Context(), g, &helper.PageRequest{
			No:       1,
			PageSize: 100,
			OrderBy:  "ROLE_NAME",
			Sort:     "ASC",
		})
		if err != nil {
			fLog.Errorf("GroupRoleRepo.ListGroupRoleByGroup got %s", err.Error())
			helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, fmt.Sprintf("subject not found : %s. got %s", authCtx.Subject, err.Error()))
			return
		}
		for _, gr := range groupRole {
			groupSummary.Roles = append(groupSummary.Roles, &RoleSummary{
				RecordID: gr.RecID,
				RoleName: gr.RoleName,
			})
		}
		whoami.Groups = append(whoami.Groups, groupSummary)
	}

	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User information populated", nil, whoami)
}

// ActivateUser serve user activation process
func ActivateUser(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "ActivateUser").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fLog.Errorf("ioutil.ReadAll got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	c := &ActivateUserRequest{}
	err = json.Unmarshal(body, c)
	if err != nil {
		fLog.Errorf("json.Unmarshal got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, "Malformed json body", nil, nil)
		return
	}
	user, err := UserRepo.GetUserByEmail(r.Context(), c.Email)
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByEmail got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	if user.ActivationCode == c.ActivationToken {
		user.Enabled = true
		err := UserRepo.SaveOrUpdate(r.Context(), user)
		if err != nil {
			fLog.Errorf("UserRepo.SaveOrUpdate got %s", err.Error())
			helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
			return
		}
		ret := make(map[string]interface{})
		ret["rec_id"] = user.RecID
		ret["email"] = user.Email
		ret["enabled"] = user.Enabled
		ret["suspended"] = user.Suspended
		ret["last_seen"] = user.LastSeen
		ret["last_login"] = user.LastLogin
		ret["2fa_enabled"] = user.Enable2FactorAuth
		helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User activated", nil, ret)
	} else {
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, "Activation token and email not match", nil, nil)
	}
}

// GetUserDetail serve fetch user detail
func GetUserDetail(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "GetUserDetail").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	ret := make(map[string]interface{})
	ret["rec_id"] = user.RecID
	ret["email"] = user.Email
	ret["enabled"] = user.Enabled
	ret["suspended"] = user.Suspended
	ret["last_seen"] = user.LastSeen
	ret["last_login"] = user.LastLogin
	ret["2fa_enabled"] = user.Enable2FactorAuth
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User retrieved", nil, ret)
}

// UpdateUserRequest hold request data for requesting to update user information.
type UpdateUserRequest struct {
	Email     string `json:"email"`
	Enabled   bool   `json:"enabled"`
	Suspended bool   `json:"suspended"`
	Enable2FA bool   `json:"2fa_enabled"`
}

// UpdateUserDetail rest endpoint to update user detail
func UpdateUserDetail(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "GetUserDetail").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	req := &UpdateUserRequest{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fLog.Errorf("ioutil.ReadAll got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}
	err = json.Unmarshal(body, req)
	if err != nil {
		fLog.Errorf("json.Unmarshal got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}

	// if email is changed and enabled = false, send email
	sendemail := false
	if user.Email != req.Email && req.Enabled == false {
		user.ActivationCode = helper.MakeRandomString(6, true, false, false, false)
		sendemail = true
	}

	if !user.Enable2FactorAuth && req.Enable2FA {
		user.UserTotpSecretKey = totp.MakeRandomTotpKey()
	}

	user.Email = req.Email
	user.Enable2FactorAuth = req.Enable2FA
	user.Enabled = req.Enabled
	user.Suspended = req.Suspended

	err = UserRepo.SaveOrUpdate(r.Context(), user)
	if err != nil {
		fLog.Errorf("UserRepo.SaveOrUpdate got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusInternalServerError, err.Error(), nil, nil)
		return
	}

	if sendemail {
		fLog.Warnf("Sending email")
		mailer.Send(r.Context(), &mailer.Email{
			From:     config.Get("mailer.from"),
			FromName: config.Get("mailer.from.name"),
			To:       []string{user.Email},
			Cc:       nil,
			Bcc:      nil,
			Template: "EMAIL_VERIFY",
			Data:     user,
		})
	}

	ret := make(map[string]interface{})
	ret["rec_id"] = user.RecID
	ret["email"] = user.Email
	ret["enabled"] = user.Enabled
	ret["suspended"] = user.Suspended
	ret["last_seen"] = user.LastSeen
	ret["last_login"] = user.LastLogin
	ret["2fa_enabled"] = user.Enable2FactorAuth
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User updated", nil, ret)

}

// DeleteUser serve user deletion
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "DeleteUser").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	UserRepo.DeleteUser(r.Context(), user)
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User deleted", nil, nil)
}

// SimpleRole define structure or request body used to list role
type SimpleRole struct {
	RecID    string `json:"rec_id"`
	RoleName string `json:"role_name"`
}

// ListUserRole serve listing all role that directly owned by user
func ListUserRole(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "ListUserRole").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/roles", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	pageRequest, err := helper.NewPageRequestFromRequest(r)
	if err != nil {
		fLog.Errorf("helper.NewPageRequestFromRequest got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	roles, page, err := UserRoleRepo.ListUserRoleByUser(r.Context(), user, pageRequest)
	if err != nil {
		fLog.Errorf("UserRoleRepo.ListUserRoleByUser got %s", err.Error())
	}
	sroles := make([]*SimpleRole, len(roles))
	for k, v := range roles {
		sroles[k] = &SimpleRole{
			RecID:    v.RecID,
			RoleName: v.RoleName,
		}
	}
	ret := make(map[string]interface{})
	ret["roles"] = sroles
	ret["page"] = page
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "List of roles paginated", nil, ret)
}

// ListAllUserRole serve listing of all roles belong to user, both direct or indirect
func ListAllUserRole(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "ListAllUserRole").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/all-roles", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	pageRequest, err := helper.NewPageRequestFromRequest(r)
	if err != nil {
		fLog.Errorf("helper.NewPageRequestFromRequest got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	roles, page, err := UserRepo.ListAllUserRoles(r.Context(), user, pageRequest)
	if err != nil {
		fLog.Errorf("UserRepo.ListAllUserRoles got %s", err.Error())
	}
	sroles := make([]*SimpleRole, len(roles))
	for k, v := range roles {
		sroles[k] = &SimpleRole{
			RecID:    v.RecID,
			RoleName: v.RoleName,
		}
	}
	ret := make(map[string]interface{})
	ret["roles"] = sroles
	ret["page"] = page
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "List of roles paginated", nil, ret)
}

// CreateUserRole serve a user-role relation
func CreateUserRole(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "CreateUserRole").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/role/{roleRecId}", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	role, err := RoleRepo.GetRoleByRecID(r.Context(), params["roleRecId"])
	if err != nil {
		fLog.Errorf("RoleRepo.GetRoleByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	_, err = UserRoleRepo.CreateUserRole(r.Context(), user, role)
	if err != nil {
		fLog.Errorf("UserRoleRepo.CreateUserRole got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User-Role created", nil, nil)
}

// DeleteUserRole serve the user deletion
func DeleteUserRole(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "DeleteUserRole").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/role/{roleRecId}", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	role, err := RoleRepo.GetRoleByRecID(r.Context(), params["roleRecId"])
	if err != nil {
		fLog.Errorf("RoleRepo.GetRoleByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	userRole, err := UserRoleRepo.GetUserRole(r.Context(), user, role)
	if err != nil {
		fLog.Errorf("UserRoleRepo.GetUserRole got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	err = UserRoleRepo.DeleteUserRole(r.Context(), userRole)
	if err != nil {
		fLog.Errorf("UserRoleRepo.DeleteUserRole got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User-Role deleted", nil, nil)
}

// ListUserGroup serve a user-group listing
func ListUserGroup(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "ListUserGroup").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/groups", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	pageRequest, err := helper.NewPageRequestFromRequest(r)
	if err != nil {
		fLog.Errorf("helper.NewPageRequestFromRequest got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	groups, page, err := UserGroupRepo.ListUserGroupByUser(r.Context(), user, pageRequest)
	if err != nil {
		fLog.Errorf("UserGroupRepo.ListUserGroupByUser got %s", err.Error())
	}
	sgroups := make([]*SimpleGroup, len(groups))
	for k, v := range groups {
		sgroups[k] = &SimpleGroup{
			RecID:     v.RecID,
			GroupName: v.GroupName,
		}
	}
	ret := make(map[string]interface{})
	ret["groups"] = sgroups
	ret["page"] = page
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "List of groups paginated", nil, ret)
}

// CreateUserGroup serve creation of user-group relation
func CreateUserGroup(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "CreateUserGroup").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/group/{groupRecId}", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	group, err := GroupRepo.GetGroupByRecID(r.Context(), params["groupRecId"])
	if err != nil {
		fLog.Errorf("GroupRepo.GetGroupByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	_, err = UserGroupRepo.CreateUserGroup(r.Context(), user, group)
	if err != nil {
		fLog.Errorf("UserGroupRepo.CreateUserGroup got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusBadRequest, err.Error(), nil, nil)
		return
	}
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User-Group created", nil, nil)
}

// DeleteUserGroup serve deleting a user-group relation
func DeleteUserGroup(w http.ResponseWriter, r *http.Request) {
	fLog := userMgmtLogger.WithField("func", "DeleteUserGroup").WithField("RequestID", r.Context().Value(constants.RequestID)).WithField("path", r.URL.Path).WithField("method", r.Method)
	params, err := helper.ParsePathParams("/api/v1/management/user/{userRecId}/group/{groupRecId}", r.URL.Path)
	if err != nil {
		panic(err)
	}
	user, err := UserRepo.GetUserByRecID(r.Context(), params["userRecId"])
	if err != nil {
		fLog.Errorf("UserRepo.GetUserByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	group, err := GroupRepo.GetGroupByRecID(r.Context(), params["groupRecId"])
	if err != nil {
		fLog.Errorf("GroupRepo.GetGroupByRecID got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	ug, err := UserGroupRepo.GetUserGroup(r.Context(), user, group)
	if err != nil {
		fLog.Errorf("UserGroupRepo.GetUserGroup got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	err = UserGroupRepo.DeleteUserGroup(r.Context(), ug)
	if err != nil {
		fLog.Errorf("UserGroupRepo.DeleteUserGroup got %s", err.Error())
		helper.WriteHTTPResponse(r.Context(), w, http.StatusNotFound, err.Error(), nil, nil)
		return
	}
	helper.WriteHTTPResponse(r.Context(), w, http.StatusOK, "User-Group deleted", nil, nil)

}
