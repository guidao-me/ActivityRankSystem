package handler

import (
	"ActivityRankSystem/service"
	"context"
)

func UpdateScore(userId string, score int64) (service.RankInfo, error) {
	ranking, err := service.New()
	if err != nil {
		return service.RankInfo{}, err
	}
	info, err := ranking.Update(context.Background(), userId, score)
	return info, err
}

func QueryRankAndScore(userId string) ([]service.RankInfo, error) {
	ranking, err := service.New()
	if err != nil {
		return []service.RankInfo{}, err
	}

	return ranking.GetRankingList(context.Background(), userId)
}
