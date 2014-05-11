// Copyright github.com/juju2013. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"

	"github.com/gogits/gogs/modules/auth/ldap"
)

// Login types.
const (
	LT_NOTYPE = iota
	LT_PLAIN
	LT_LDAP
	LT_SMTP
)

var (
	ErrAuthenticationAlreadyExist = errors.New("Authentication already exist")
	ErrAuthenticationNotExist     = errors.New("Authentication does not exist")
	ErrAuthenticationUserUsed     = errors.New("Authentication has been used by some users")
)

var LoginTypes = map[int]string{
	LT_LDAP: "LDAP",
	LT_SMTP: "SMTP",
}

var _ core.Conversion = &LDAPConfig{}

type LDAPConfig struct {
	ldap.Ldapsource
}

// implement
func (cfg *LDAPConfig) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &cfg.Ldapsource)
}

func (cfg *LDAPConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg.Ldapsource)
}

type LoginSource struct {
	Id                int64
	Type              int
	Name              string          `xorm:"unique"`
	IsActived         bool            `xorm:"not null default false"`
	Cfg               core.Conversion `xorm:"TEXT"`
	Created           time.Time       `xorm:"created"`
	Updated           time.Time       `xorm:"updated"`
	AllowAutoRegisted bool            `xorm:"not null default false"`
}

func (source *LoginSource) TypeString() string {
	return LoginTypes[source.Type]
}

func (source *LoginSource) LDAP() *LDAPConfig {
	return source.Cfg.(*LDAPConfig)
}

// for xorm callback
func (source *LoginSource) BeforeSet(colName string, val xorm.Cell) {
	if colName == "type" {
		ty := (*val).(int64)
		switch ty {
		case LT_LDAP:
			source.Cfg = new(LDAPConfig)
		}
	}
}

func GetAuths() ([]*LoginSource, error) {
	var auths = make([]*LoginSource, 0)
	err := orm.Find(&auths)
	return auths, err
}

func GetLoginSourceById(id int64) (*LoginSource, error) {
	source := new(LoginSource)
	has, err := orm.Id(id).Get(source)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrAuthenticationNotExist
	}
	return source, nil
}

func AddLDAPSource(name string, cfg *LDAPConfig) error {
	_, err := orm.Insert(&LoginSource{Type: LT_LDAP,
		Name:      name,
		IsActived: true,
		Cfg:       cfg,
	})
	return err
}

func UpdateLDAPSource(source *LoginSource) error {
	_, err := orm.AllCols().Id(source.Id).Update(source)
	return err
}

func DelLoginSource(source *LoginSource) error {
	cnt, err := orm.Count(&User{LoginSource: source.Id})
	if err != nil {
		return err
	}
	if cnt > 0 {
		return ErrAuthenticationUserUsed
	}
	_, err = orm.Id(source.Id).Delete(&LoginSource{})
	return err
}

// login a user
func LoginUser(uname, passwd string) (*User, error) {
	var u *User
	var emailLogin bool
	if strings.Contains(uname, "@") {
		u = &User{Email: uname}
		emailLogin = true
	} else {
		u = &User{LowerName: strings.ToLower(uname)}
	}

	has, err := orm.Get(u)
	if err != nil {
		return nil, err
	}

	// if email login, then we cannot auto register
	if emailLogin {
		if !has {
			return nil, ErrUserNotExist
		}
	}
	if u.LoginType == LT_NOTYPE {
		u.LoginType = LT_PLAIN
	}

	// for plain login, user must have existed.
	if u.LoginType == LT_PLAIN {
		if !has {
			return nil, ErrUserNotExist
		}

		newUser := &User{Passwd: passwd, Salt: u.Salt}
		newUser.EncodePasswd()
		if u.Passwd != newUser.Passwd {
			return nil, ErrUserNotExist
		}
		return u, nil
	} else {
		if !has {
			var sources []LoginSource
			cond := &LoginSource{IsActived: true, AllowAutoRegisted: true}
			err = orm.UseBool().Find(&sources, cond)
			if err != nil {
				return nil, err
			}

			for _, source := range sources {
				u, err := LoginUserLdapSource(nil, u.LoginName, passwd,
					source.Id, source.Cfg.(*LDAPConfig), true)
				if err == nil {
					return u, err
				}
			}

			return nil, ErrUserNotExist
		}

		var source LoginSource
		hasSource, err := orm.Id(u.LoginSource).Get(&source)
		if err != nil {
			return nil, err
		}
		if !hasSource {
			return nil, ErrLoginSourceNotExist
		}

		if !source.IsActived {
			return nil, ErrLoginSourceNotActived
		}

		switch u.LoginType {
		case LT_LDAP:
			return LoginUserLdapSource(u, u.LoginName, passwd,
				source.Id, source.Cfg.(*LDAPConfig), false)
		case LT_SMTP:
		}
		return nil, ErrUnsupportedLoginType
	}
}

// Query if name/passwd can login against the LDAP direcotry pool
// Create a local user if success
// Return the same LoginUserPlain semantic
func LoginUserLdapSource(user *User, name, passwd string, sourceId int64, cfg *LDAPConfig, autoRegister bool) (*User, error) {
	mail, logged := cfg.Ldapsource.SearchEntry(name, passwd)
	if !logged {
		// user not in LDAP, do nothing
		return nil, ErrUserNotExist
	}
	if !autoRegister {
		return user, nil
	}

	// fake a local user creation
	user = &User{
		LowerName:   strings.ToLower(name),
		Name:        strings.ToLower(name),
		LoginType:   LT_LDAP,
		LoginSource: sourceId,
		LoginName:   name,
		IsActive:    true,
		Passwd:      passwd,
		Email:       mail,
	}

	return RegisterUser(user)
}