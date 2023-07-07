package main

import (
	"ActivityRankSystem/ctrl"
	"ActivityRankSystem/gredis"
	"github.com/gin-gonic/gin"
)

func init() {
	gredis.SetUp()
}

func main() {
	router := gin.Default()

	// 增加排行榜分数
	router.POST("/rank/update", ctrl.UpdateScore)

	// 查询玩家排行(玩家及前后10位)
	router.GET("/rank/query", ctrl.QueryRankAndScore)

	// 默认8080端口
	router.Run()
}
