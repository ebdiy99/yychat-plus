package handler

import (
	"chatplus/core/types"
	"chatplus/store/model"
	"chatplus/store/vo"
	"chatplus/utils"
	"chatplus/utils/resp"

	"github.com/gin-gonic/gin"
)

// Update 更新会话标题
func (h *ChatHandler) Update(c *gin.Context) {
	var data struct {
		Id    uint   `json:"id"`
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	var m = model.ChatItem{}
	m.Id = data.Id
	res := h.db.Model(&m).UpdateColumn("title", data.Title)
	if res.Error != nil {
		resp.ERROR(c, "Failed to update database")
		return
	}

	resp.SUCCESS(c, types.OkMsg)
}

// Remove 删除会话
func (h *ChatHandler) Remove(c *gin.Context) {
	chatId := h.GetTrim(c, "chat_id")
	if chatId == "" {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	user, err := utils.GetLoginUser(c, h.db)
	if err != nil {
		resp.NotAuth(c)
		return
	}

	res := h.db.Where("user_id = ? AND chat_id = ?", user.Id, chatId).Delete(&model.ChatItem{})
	if res.Error != nil {
		resp.ERROR(c, "Failed to update database")
		return
	}

	// 清空会话上下文
	h.App.ChatContexts.Delete(chatId)
	resp.SUCCESS(c, types.OkMsg)
}

// History 获取聊天历史记录
func (h *ChatHandler) History(c *gin.Context) {
	chatId := c.Query("chat_id") // 会话 ID
	user, err := utils.GetLoginUser(c, h.db)
	if err != nil {
		resp.NotAuth(c)
		return
	}
	var items []model.HistoryMessage
	var messages = make([]vo.HistoryMessage, 0)
	res := h.db.Where("chat_id = ? AND user_id = ?", chatId, user.Id).Find(&items)
	if res.Error != nil {
		resp.ERROR(c, "No history message")
		return
	} else {
		for _, item := range items {
			var v vo.HistoryMessage
			err := utils.CopyObject(item, &v)
			v.CreatedAt = item.CreatedAt.Unix()
			v.UpdatedAt = item.UpdatedAt.Unix()
			if err == nil {
				messages = append(messages, v)
			}
		}
	}

	resp.SUCCESS(c, messages)
}

// Clear 清空所有聊天记录
func (h *ChatHandler) Clear(c *gin.Context) {
	// 获取当前登录用户所有的聊天会话
	user, err := utils.GetLoginUser(c, h.db)
	if err != nil {
		resp.NotAuth(c)
		return
	}

	var chats []model.ChatItem
	res := h.db.Where("user_id = ?", user.Id).Find(&chats)
	if res.Error != nil {
		resp.ERROR(c, "No chats found")
		return
	}
	// 清空聊天记录
	for _, chat := range chats {
		err := h.db.Where("chat_id = ? AND user_id = ?", chat.ChatId, user.Id).Delete(&model.HistoryMessage{})
		if err != nil {
			logger.Warnf("Failed to delele chat history for ChatID: %s", chat.ChatId)
		}
		// 清空会话上下文
		h.App.ChatContexts.Delete(chat.ChatId)
	}
	// 删除所有的会话记录
	res = h.db.Where("user_id = ?", user.Id).Delete(&model.ChatItem{})
	if res.Error != nil {
		resp.ERROR(c, "Failed to remove chat from database.")
		return
	}

	resp.SUCCESS(c, types.OkMsg)
}
