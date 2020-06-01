package model

import "github.com/google/jsonapi"

type Token struct {
    ID          uint        `jsonapi:"primary,token" gorm:"primary_key"`
    Secret      *string     `jsonapi:"attr,secret" gorm:"not null"`
    // Status codes:
    // - created :  newly created, not linked to any NUSID
    // - active  :  user has authenticated himself through OpenID
    // - expired
    // - destroyed
    Status      *string     `jsonapi:"attr,status" gorm:"not null;default:'created'"`

    IdentityFields
    DBTime

    // URL redirecting user to openid.nus.edu.sg to authenticate himself
    AuthURL    string          `gorm:"-"`
    jsonapi.Linkable
}

func (token Token) JSONAPILinks() *jsonapi.Links {
    return &jsonapi.Links{
        "auth": token.AuthURL,
    }
}