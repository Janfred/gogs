// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1

import (
	api "github.com/gogits/go-gogs-client"

	"github.com/gogits/gogs/models"
	"github.com/gogits/gogs/modules/middleware"
)

// https://github.com/gogits/go-gogs-client/wiki/Users#list-access-tokens-for-a-user
func ListAccessTokens(ctx *middleware.Context) {
	tokens, err := models.ListAccessTokens(ctx.User.Id)
	if err != nil {
		ctx.APIError(500, "ListAccessTokens", err)
		return
	}

	apiTokens := make([]*api.AccessToken, len(tokens))
	for i := range tokens {
		apiTokens[i] = &api.AccessToken{tokens[i].Name, tokens[i].Sha1}
	}
	ctx.JSON(200, &apiTokens)
}

type CreateAccessTokenForm struct {
	Name string `json:"name" binding:"Required"`
}

// https://github.com/gogits/go-gogs-client/wiki/Users#create-a-access-token
func CreateAccessToken(ctx *middleware.Context, form CreateAccessTokenForm) {
	t := &models.AccessToken{
		UID:  ctx.User.Id,
		Name: form.Name,
	}
	if err := models.NewAccessToken(t); err != nil {
		ctx.APIError(500, "NewAccessToken", err)
		return
	}
	ctx.JSON(201, &api.AccessToken{t.Name, t.Sha1})
}
