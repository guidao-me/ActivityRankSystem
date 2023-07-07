package service

import (
	"ActivityRankSystem/gredis"
	"ActivityRankSystem/util"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"time"
)

var RankKey = "ar:[timeStr]"

// ZRanking 排行榜：
// 使用redis zset，得分相同时，按时间先后进行排序，用户排行名次Rank从1开始；
// 将zset score按十进制数拆分，score十进制数字总共固定为16位（超过16位的数会有浮点数精度导致进位的问题），
// 整数部分用于表示用户排序值val，小数部分表示排行活动结束时间戳（秒）与用户排序值更新时间戳（秒）的差值deltaTs，
// 小数部分的数字长度由deltaTs的数字长度确定，整数部分最大支持长度则为：16-len(deltaTs)。
// 比如活动时长为10天，总时间差为864000，长度为6，则deltaTs宽度为6，不够则在前面补0
type ZRanking struct {
	Key            string // redis zset key
	StartTimestamp int64  // 排行活动开始时间
	EndTimestamp   int64  // 排行活动结束时间戳
	TimePadWidth   int    // 排行榜活动结束时间与用户排序值更新时间的差值补0宽度
}

// RankInfo 排行用户信息
type RankInfo struct {
	UID  string // 用户id
	Val  int64  // 用户排行值
	Rank int64  // 用户排名
}

// New 创建ZRanking实例
func New() (*ZRanking, error) {
	key := strings.ReplaceAll(RankKey, "[timeStr]", util.GetCurYearAndMonthStr())
	startTs := util.GetFirstDateOfMonth(time.Now()).Unix()
	endTs := util.GetLastDateOfMonth(time.Now()).Unix()
	deltaTs := endTs - startTs
	if deltaTs <= 0 {
		return nil, fmt.Errorf("invalid deltaTs:%v", deltaTs)
	}
	timePadWidth := len(fmt.Sprint(deltaTs))
	return &ZRanking{
		Key:            key,
		StartTimestamp: startTs,
		EndTimestamp:   endTs,
		TimePadWidth:   timePadWidth,
	}, nil
}

// Update 更新排行榜
func (r *ZRanking) Update(ctx context.Context, uid string, val int64) (RankInfo, error) {
	valScore, err := r.val2score(ctx, val)
	if err != nil {
		err = errors.Wrap(err, "ZRanking Update val2score error")
		return RankInfo{}, err
	}

	keys := []string{r.Key}
	args := []interface{}{uid, valScore}
	zincrby := redis.NewScript(`
-- 排行榜key
local key = KEYS[1]
-- 要更新的用户id
local uid = ARGV[1]
-- 用户本次新增的val（小数位为时间差标识）
local valScore = ARGV[2]

-- 获取用户之前的score
local score = redis.call("ZSCORE", key, uid)
if score == false then
    score = 0
end
-- 从score中抹除用于时间差标识的小数部分，获取整数的排序val
local val = math.floor(score)

-- 更新用户最新的score信息（累计val.最新时间差）
local newScore = valScore+val
redis.call("ZADD", key, newScore, uid)

-- 更新成功返回newScore（注意要使用tostring才能返回小数）
return tostring(newScore)
	`)
	newScore, err := zincrby.Run(ctx, gredis.MainClient, keys, args...).Float64()
	if err != nil {
		err = errors.Wrap(err, "ZRanking Update Run lua error")
		return RankInfo{}, err
	}

	rank := gredis.MainClient.ZRevRank(ctx, r.Key, uid).Val()

	score, err := r.score2val(ctx, newScore)
	if err != nil {
		return RankInfo{}, err
	}

	return RankInfo{
		UID:  uid,
		Val:  score,
		Rank: rank + 1,
	}, nil
}

// val 转为 score:
// score = float64(val.deltaTs)
func (r *ZRanking) val2score(ctx context.Context, val int64) (float64, error) {
	nowts := time.Now().Unix()
	deltaTs := r.EndTimestamp - nowts
	scoreFormat := fmt.Sprintf("%%v.%%0%dd", r.TimePadWidth)
	scoreStr := fmt.Sprintf(scoreFormat, val, deltaTs)
	score, err := strconv.ParseFloat(scoreStr, 64)
	if err != nil {
		err = errors.Wrap(err, "ZRanking val2score ParseFloat error")
		return 0, err
	}
	return score, nil
}

// 从 score 中获取 val
func (r *ZRanking) score2val(ctx context.Context, score float64) (int64, error) {
	scoreStr := fmt.Sprint(score)
	ss := strings.Split(scoreStr, ".")
	valStr := ss[0]
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		err = errors.Wrap(err, "ZRanking score2val ParseInt error")
		return 0, err
	}
	return val, nil
}

// GetRankingList 返回排行榜
// topN <= 0 取全量
// desc 是否按score降序排列
func (r *ZRanking) GetRankingList(ctx context.Context, userId string) ([]RankInfo, error) {
	result := []RankInfo{}
	rank, err := r.GetUserRank(ctx, userId, true)
	if err != nil {
		return result, err
	}
	zRevRange := gredis.MainClient.ZRevRangeWithScores
	startIdx := rank - 10
	if startIdx < 0 {
		startIdx = 0
	}
	list, err := zRevRange(ctx, r.Key, startIdx, rank+10).Result()
	if err != nil {
		return nil, err
	}
	for idx, z := range list {
		val, err := r.score2val(ctx, z.Score)
		if err != nil {
			return nil, errors.Wrapf(err, "ZRanking GetRankingList score2val error, uid:%v score:%v", z.Member, z.Score)
		}
		member := z.Member.(string)
		m := RankInfo{
			UID:  member,
			Val:  val,
			Rank: int64(idx + 1),
		}
		result = append(result, m)
	}
	return result, nil
}

// GetUserRank 获取某个用户的排行
func (r *ZRanking) GetUserRank(ctx context.Context, uid string, desc bool) (int64, error) {
	zrank := gredis.MainClient.ZRank
	if desc {
		zrank = gredis.MainClient.ZRevRank
	}
	idx, err := zrank(ctx, r.Key, fmt.Sprint(uid)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return idx, err
}

// GetUserVal 获取某个用户score中的排序值
func (r *ZRanking) GetUserVal(ctx context.Context, uid int64) (int64, error) {
	score, err := gredis.MainClient.ZScore(ctx, r.Key, fmt.Sprint(uid)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return r.score2val(ctx, score)
}

// GetTotalCount 获取排行榜总人数
func (r *ZRanking) GetTotalCount(ctx context.Context) int64 {
	return gredis.MainClient.ZCard(ctx, r.Key).Val()
}
