package helper

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"

	"github.com/SermoDigital/jose/crypto"
	"github.com/SermoDigital/jose/jws"
)

type HansipToken struct {
	Issuer     string
	Subject    string
	Audiences  []string
	Expire     time.Time
	NotBefore  time.Time
	IssuedAt   time.Time
	Additional map[string]interface{}
	Token      string
}

// TokenFactory defines a token factory function to implement
type TokenFactory interface {
	CreateTokenPair(subject string, audience []string, additional map[string]interface{}) (string, string, error)
	ReadToken(token string) (*HansipToken, error)
	RefreshToken(refreshToken string) (string, error)
}

// NewTokenFactory create new instance of TokenFactory
func NewTokenFactory(signKey, signMethod, issuer string, accessTokenAge, refreshTokenAge time.Duration) TokenFactory {
	if issuer == "" {
		panic("empty issuer")
	}
	return &DefaultTokenFactory{
		Issuer:               issuer,
		AccessTokenDuration:  accessTokenAge,
		RefreshTokenDuration: refreshTokenAge,
		SignKey:              signKey,
		SignMethod:           signMethod,
	}
}

// DefaultTokenFactory default implementation of TokenFactory
type DefaultTokenFactory struct {
	mutex                sync.Mutex
	Issuer               string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	SignKey              string
	SignMethod           string
}

// CreateTokenPair create new Access and Refresh token pair
func (tf *DefaultTokenFactory) CreateTokenPair(subject string, audience []string, additional map[string]interface{}) (string, string, error) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()
	accessAdditional := make(map[string]interface{})
	refreshAdditional := make(map[string]interface{})
	if additional != nil {
		for k, v := range additional {
			accessAdditional[k] = v
			refreshAdditional[k] = v
		}
	}
	accessAdditional["type"] = "access"
	refreshAdditional["type"] = "refresh"

	access, err := CreateJWTStringToken(tf.SignKey, tf.SignMethod, tf.Issuer, subject, audience, time.Now(), time.Now(), time.Now().Add(tf.AccessTokenDuration), accessAdditional)
	if err != nil {
		return "", "", err
	}
	refresh, err := CreateJWTStringToken(tf.SignKey, tf.SignMethod, tf.Issuer, subject, audience, time.Now(), time.Now(), time.Now().Add(tf.RefreshTokenDuration), refreshAdditional)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// ReadToken read a token string, validate and extract its content.
func (tf *DefaultTokenFactory) ReadToken(token string) (*HansipToken, error) {
	issuer, subject, audience, issuedAt, notBefore, expire, additional, err := ReadJWTStringToken(true, tf.SignKey, tf.SignMethod, token)
	htoken := &HansipToken{
		Issuer:     issuer,
		Subject:    subject,
		Audiences:  audience,
		Expire:     expire,
		NotBefore:  notBefore,
		IssuedAt:   issuedAt,
		Additional: additional,
		Token:      token,
	}
	if issuer != tf.Issuer {
		return htoken, fmt.Errorf("invalid issuer %s", issuer)
	}
	return htoken, err
}

// RefreshToken generate new Access token by specifying its refresh token
func (tf *DefaultTokenFactory) RefreshToken(refreshToken string) (string, error) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()
	hToken, err := tf.ReadToken(refreshToken)
	if err != nil {
		return "", err
	}
	if hToken.Issuer != tf.Issuer {
		return "", fmt.Errorf("invalid issuer")
	}
	if typ, ok := hToken.Additional["type"]; ok {
		if typ != "refresh" {
			return "", fmt.Errorf("not refresh token")
		}
	} else {
		return "", fmt.Errorf("unknown token type")
	}
	hToken.Additional["type"] = "access"
	access, err := CreateJWTStringToken(tf.SignKey, tf.SignMethod, tf.Issuer, hToken.Subject, hToken.Audiences, hToken.IssuedAt, hToken.NotBefore, time.Now().Add(tf.AccessTokenDuration), hToken.Additional)
	if err != nil {
		return "", err
	}
	return access, err
}

// ReadJWTStringToken takes a token string , keys, signMethod and returns its content.
func ReadJWTStringToken(validate bool, signKey, signMethod, tokenString string) (string, string, []string, time.Time, time.Time, time.Time, map[string]interface{}, error) {
	if signKey == "th15mustb3CH@ngedINprodUCT10N" {
		logrus.Warnf("Using default CryptKey for JWT Token, This key is visible from the source tree and to be used in development only. YOU MUST CHANGE THIS IN PRODUCTION or TO REMOVE THIS LOG FROM APPEARING")
	}

	jwt, err := jws.ParseJWT([]byte(tokenString))
	if err != nil {
		return "", "", nil, time.Now(), time.Now(), time.Now(), nil, fmt.Errorf("malformed jwt token")
	}

	if validate {
		var sMethod crypto.SigningMethod

		switch strings.ToUpper(signMethod) {
		case "HS256":
			sMethod = crypto.SigningMethodHS256
		case "HS384":
			sMethod = crypto.SigningMethodHS384
		case "HS512":
			sMethod = crypto.SigningMethodHS512
		default:
			sMethod = crypto.SigningMethodHS256
		}

		if err := jwt.Validate([]byte(signKey), sMethod); err != nil {
			return "", "", nil, time.Now(), time.Now(), time.Now(), nil, fmt.Errorf("invalid jwt token - %s", err.Error())
		}
	}
	claims := jwt.Claims()
	additional := make(map[string]interface{})
	for k, v := range claims {
		kup := strings.ToUpper(k)
		if kup != "ISS" && kup != "AUD" && kup != "SUB" && kup != "IAT" && kup != "EXP" && kup != "NBF" {
			additional[k] = v
		}
	}

	issuer, _ := claims.Issuer()
	subject, _ := claims.Subject()
	audience, _ := claims.Audience()
	expire, _ := claims.Expiration()
	notBefore, _ := claims.NotBefore()
	issuedAt, _ := claims.IssuedAt()

	return issuer, subject, audience, issuedAt, notBefore, expire, additional, nil
}

// CreateJWTStringToken create JWT String token based on arguments
func CreateJWTStringToken(signKey, signMethod, issuer, subject string, audience []string, issuedAt, notBefore, expiration time.Time, additional map[string]interface{}) (string, error) {
	if signKey == "th15mustb3CH@ngedINprodUCT10N" {
		logrus.Warnf("Using default CryptKey for JWT Token, This key is visible from the source tree and to be used in development only. YOU MUST CHANGE THIS IN PRODUCTION or TO REMOVE THIS LOG FROM APPEARING")
	}

	claims := jws.Claims{}
	claims.SetIssuer(issuer)
	claims.SetSubject(subject)
	claims.SetAudience(audience...)
	claims.SetIssuedAt(issuedAt)
	claims.SetNotBefore(notBefore)
	claims.SetExpiration(expiration)

	for k, v := range additional {
		claims[k] = v
	}

	var signM crypto.SigningMethod

	switch strings.ToUpper(signMethod) {
	case "HS256":
		signM = crypto.SigningMethodHS256
	case "HS384":
		signM = crypto.SigningMethodHS384
	case "HS512":
		signM = crypto.SigningMethodHS512
	default:
		signM = crypto.SigningMethodHS256
	}

	jwtBytes := jws.NewJWT(claims, signM)

	tokenByte, err := jwtBytes.Serialize([]byte(signKey))
	if err != nil {
		panic(err)
	}
	return string(tokenByte), nil
}
