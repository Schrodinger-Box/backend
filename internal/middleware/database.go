package middleware

import (
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"

	"schrodinger-box/internal/model"
)

func DatabaseMiddleware(connString string) gin.HandlerFunc {
	db, err := gorm.Open("mysql", connString)
	if err != nil {
		panic("Fail to connect to DB: " + err.Error())
	}

	// Migrating table
	tables := []interface{}{
		model.Token{},
		model.User{},
	}
	db.AutoMigrate(tables...)

	return func(ctx *gin.Context) {
		ctx.Set("DB", db)
		ctx.Next()
	}
}
