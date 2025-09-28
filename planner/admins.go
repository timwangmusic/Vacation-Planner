package planner

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/user"
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

	adminView, authErr := p.UserAuthentication(ctx, user.LevelAdmin)
	if authErr != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": authErr.GetErrorMessage()})
		return
	}

	var announcement Announcement
	if err := ctx.ShouldBindJSON(&announcement); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	announcement.AdminID = adminView.ID
	announcement.ID = uuid.NewString()
	announcement.Timestamp = time.Now().Format(time.RFC3339)

	if err := p.Mailer.Broadcast(ctx, announcement.Subject, announcement.Message, string(p.Environment)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data, marshalErr := json.Marshal(announcement)
	if marshalErr != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": marshalErr.Error()})
		return
	}

	if err := p.RedisClient.SaveAnnouncement(ctx, announcement.ID, string(data)); err != nil {
		iowrappers.Logger.Error(err)
	}

	ctx.JSON(http.StatusOK, announcement)
}
