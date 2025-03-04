package handler

import (
	"chatplus/store/model"
	"chatplus/store/vo"
	"chatplus/utils"
	"chatplus/utils/resp"
	"github.com/gin-gonic/gin"
)

// List 获取会话列表
func (h *ChatHandler) List(c *gin.Context) {
	userId := h.GetInt(c, "user_id", 0)
	if userId == 0 {
		resp.ERROR(c, "The parameter 'user_id' is needed.")
		return
	}
	var items = make([]vo.ChatItem, 0)
	var chats []model.ChatItem
	res := h.db.Where("user_id = ?", userId).Order("id DESC").Find(&chats)
	if res.Error == nil {
		var roleIds = make([]uint, 0)
		for _, chat := range chats {
			roleIds = append(roleIds, chat.RoleId)
		}
		var roles []model.ChatRole
		res = h.db.Find(&roles, roleIds)
		if res.Error == nil {
			roleMap := make(map[uint]model.ChatRole)
			for _, role := range roles {
				roleMap[role.Id] = role
			}

			for _, chat := range chats {
				var item vo.ChatItem
				err := utils.CopyObject(chat, &item)
				if err == nil {
					item.Id = chat.Id
					item.Icon = roleMap[chat.RoleId].Icon
					items = append(items, item)
				}
			}
		}

	}
	resp.SUCCESS(c, items)
}

func (h *ChatHandler) Detail(c *gin.Context) {
	chatId := h.GetTrim(c, "chat_id")
	if utils.IsEmptyValue(chatId) {
		resp.ERROR(c, "Invalid chatId")
		return
	}

	var chatItem model.ChatItem
	res := h.db.Where("chat_id = ?", chatId).First(&chatItem)
	if res.Error != nil {
		resp.ERROR(c, "No chat found")
		return
	}

	var chatItemVo vo.ChatItem
	err := utils.CopyObject(chatItem, &chatItemVo)
	if err != nil {
		resp.ERROR(c, err.Error())
		return
	}

	resp.SUCCESS(c, chatItemVo)
}
