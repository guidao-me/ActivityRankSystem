package ctrl

import (
	"ActivityRankSystem/handler"
	"github.com/gin-gonic/gin"
	"net/http"
)

// 增加玩家排行分数
func UpdateScore(c *gin.Context) {
	var req struct {
		UserId string `json:"userId"`
		Score  int64  `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "unauthorized"})
		return
	}

	result, err := handler.UpdateScore(req.UserId, req.Score)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// 查询排行和分数
func QueryRankAndScore(c *gin.Context) {
	userId := c.Query("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "unauthorized"})
		return
	}

	infos, err := handler.QueryRankAndScore(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, infos)
}
