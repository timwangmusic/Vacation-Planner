package planner

import (
	"encoding/json"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
	"net/http"
	"time"
)

type Announcement struct {
	ID        string `json:"id"`
	AdminID   string `json:"admin_id"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func (p *MyPlanner) announce(ctx *gin.Context) {
	requestId := requestid.Get(ctx)
	ctx.Set(requestIdKey, requestId)

	adminView, err := p.UserAuthentication(ctx, user.LevelAdmin)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err})
		return
	}

	var announcement Announcement
	err = ctx.ShouldBindJSON(&announcement)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	announcement.AdminID = adminView.ID
	announcement.ID = uuid.NewString()
	announcement.Timestamp = time.Now().Format(time.RFC3339)

	if err = p.Mailer.Broadcast(ctx, announcement.Subject, announcement.Message, string(p.Environment)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data, marshalErr := json.Marshal(announcement)
	if marshalErr != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = p.RedisClient.SaveAnnouncement(ctx, announcement.ID, string(data))
	if err != nil {
		iowrappers.Logger.Error(err)
	}

	ctx.JSON(http.StatusOK, announcement)
}
