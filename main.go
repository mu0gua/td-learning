package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Arman92/go-tdlib"
)

var client *tdlib.Client
var allChats []*tdlib.Chat
var haveFullChatList bool
var download []*tdlib.File

func main() {
	run()

}
func run() {
	tdlib.SetLogVerbosityLevel(1)
	tdlib.SetFilePath("./errors.txt")

	// Create new instance of client
	client = tdlib.NewClient(tdlib.Config{
		APIID:               "",
		APIHash:             "",
		SystemLanguageCode:  "en",
		DeviceModel:         "Server",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
		UseMessageDatabase:  true,
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseTestDataCenter:   false,
		DatabaseDirectory:   "./tdlib-db",
		FileDirectory:       "./tdlib-files",
		IgnoreFileNames:     false,
	})

	// Handle Ctrl+C , Gracefully exit and shutdown tdlib
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		for _, file := range download {
			for !file.Local.IsDownloadingCompleted {
				fmt.Println("file download: %s ", file.ID)
				time.Sleep(5 * time.Second)
				continue
			}
		}
		client.DestroyInstance()
		fmt.Println("shutdown tdlib and exit.")
		os.Exit(1)

	}()

	addProxy(client)
	Auth(client)
	getChatList(client, 100)

	fmt.Printf("Get %d chats\n", len(allChats))

	// var chatid int64 = x
	// var fromindex int64 = x
	// var offset int32 = -99
	// var limit int32 = 100

	// getHistoryByChat(client, chatid, fromindex, offset, limit)

	//var chatid int64 = x
	getRawMessage(client, 100)

	for {
		time.Sleep(1 * time.Second)
	}

}

func addProxy(client *tdlib.Client) {
	//add proxy
	client.AddProxy("x", 10808, true, tdlib.NewProxyTypeSocks5("", ""))
}

func Auth(client *tdlib.Client) {
	for {
		currentState, _ := client.Authorize()
		if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPhoneNumberType {
			fmt.Print("Enter phone: ")
			var number string
			fmt.Scanln(&number)
			_, err := client.SendPhoneNumber(number)
			if err != nil {
				fmt.Printf("Error sending phone number: %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitCodeType {
			fmt.Print("Enter code: ")
			var code string
			fmt.Scanln(&code)
			_, err := client.SendAuthCode(code)
			if err != nil {
				fmt.Printf("Error sending auth code : %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateWaitPasswordType {
			fmt.Print("Enter Password: ")
			var password string
			fmt.Scanln(&password)
			_, err := client.SendAuthPassword(password)
			if err != nil {
				fmt.Printf("Error sending auth password: %v", err)
			}
		} else if currentState.GetAuthorizationStateEnum() == tdlib.AuthorizationStateReadyType {
			fmt.Println("Authorization Ready! Let's rock")
			break
		}
	}
}

// see https://stackoverflow.com/questions/37782348/how-to-use-getchats-in-tdlib
func getChatList(client *tdlib.Client, limit int) error {

	if !haveFullChatList && limit > len(allChats) {
		offsetOrder := int64(math.MaxInt64)
		offsetChatID := int64(0)
		var lastChat *tdlib.Chat

		if len(allChats) > 0 {
			lastChat = allChats[len(allChats)-1]
			offsetOrder = int64(lastChat.Order)
			offsetChatID = lastChat.ID
		}

		// get chats (ids) from tdlib
		chats, err := client.GetChats(tdlib.JSONInt64(offsetOrder),
			offsetChatID, int32(limit-len(allChats)))
		if err != nil {
			return err
		}
		if len(chats.ChatIDs) == 0 {
			haveFullChatList = true
			return nil
		}

		for _, chatID := range chats.ChatIDs {
			// get chat info from tdlib
			chat, err := client.GetChat(chatID)
			if err == nil {
				allChats = append(allChats, chat)
			} else {
				return err
			}
		}

		return getChatList(client, limit)
	}
	return nil
}

func getHistoryByChat(client *tdlib.Client, chatid int64, frommsg int64, offset int32, limit int32) {

	tdmsg, err := client.GetChatHistory(chatid, frommsg, offset, limit, false)
	if err != nil {
		fmt.Printf("Get chat %d History error. \n", chatid)
		fmt.Println(err)
		return
	}

	fmt.Println("Count Message:", tdmsg.TotalCount)
	for _, msg := range tdmsg.Messages {
		fmt.Printf("Chat: %d, MessageId: %d, Message Type: %s \n", msg.ChatID, msg.ID, msg.MessageType())

		tdmsg, err := client.GetMessage(msg.ChatID, msg.ID)
		if err != nil {
			fmt.Printf("Get chat Message error. Chat: %d, Message: %d \n", msg.ChatID, msg.ID)
			fmt.Println(err)
			return
		}
		getMessageType(tdmsg)
	}
}

func getMessageType(msg *tdlib.Message) {

	msgByte, err := json.Marshal(msg.Content)
	if err != nil {
		fmt.Printf("parse json error. Chat: %d, Message: %d \n", msg.ChatID, msg.ID)
		fmt.Println(err)
		return
	}
	var priority int32 = 32
	switch msg.Content.GetMessageContentEnum() {
	case tdlib.MessageTextType:
		var msgText tdlib.MessageText
		json.Unmarshal(msgByte, &msgText)
		fmt.Printf("User: %d, message: %s \n", msg.SenderUserID, strings.ReplaceAll(msgText.Text.Text, "\n", "\\n"))

	case tdlib.MessageAnimationType:
		var msgAnimation tdlib.MessageAnimation
		json.Unmarshal(msgByte, &msgAnimation)
		fmt.Printf("message file: %s, download. size: %d , type: %s \n", msgAnimation.Animation.FileName, msgAnimation.Animation.Animation.Size, msgAnimation.Type)
		saveFileById(msgAnimation.Animation.Animation.ID, priority)

	case tdlib.MessageAudioType:
		var msgAudio tdlib.MessageAudio
		json.Unmarshal(msgByte, &msgAudio)
		fmt.Printf("message file: %s, download. size: %d , type: %s \n", msgAudio.Audio.FileName, msgAudio.Audio.Audio.Size, msgAudio.Type)
		saveFileById(msgAudio.Audio.Audio.ID, priority)

	case tdlib.MessageDocumentType:
		var msgDocument tdlib.MessageDocument
		json.Unmarshal(msgByte, &msgDocument)
		fmt.Printf("message file: %s, download. size: %d , type: %s \n", msgDocument.Document.FileName, msgDocument.Document.Document.Size, msgDocument.Type)
		saveFileById(msgDocument.Document.Document.ID, priority)

	case tdlib.MessagePhotoType:
		var msgPhoto tdlib.MessagePhoto
		json.Unmarshal(msgByte, &msgPhoto)
		fmt.Printf("message photo file: %s", string(msgByte))

	case tdlib.MessageExpiredPhotoType:
		var msgExpiredPhoto tdlib.MessageExpiredPhoto
		json.Unmarshal(msgByte, &msgExpiredPhoto)

	case tdlib.MessageVoiceNoteType:
		var msgVoiceNote tdlib.MessageVoiceNote
		json.Unmarshal(msgByte, &msgVoiceNote)

		fmt.Printf("message VoiceNote file: %s", string(msgByte))

	case tdlib.MessageStickerType:
		var msgSticker tdlib.MessageSticker
		json.Unmarshal(msgByte, &msgSticker)

		fmt.Printf("message Sticker file: %s", string(msgByte))

	case tdlib.MessageVideoType:
		var msgVideo tdlib.MessageVideo
		json.Unmarshal(msgByte, &msgVideo)

		fmt.Printf("message file: %s, download. size: %d , type: %s \n", msgVideo.Video.FileName, msgVideo.Video.Video.Size, msgVideo.Type)
		saveFileById(msgVideo.Video.Video.ID, priority)

	case tdlib.MessageExpiredVideoType:
		var msgExpiredVideo tdlib.MessageExpiredVideo
		json.Unmarshal(msgByte, &msgExpiredVideo)

	case tdlib.MessageVideoNoteType:
		var msgVideoNote tdlib.MessageVideoNote
		json.Unmarshal(msgByte, &msgVideoNote)

	case tdlib.MessageLocationType:
		var msgLocation tdlib.MessageLocation
		json.Unmarshal(msgByte, &msgLocation)
	case tdlib.MessageVenueType:
		var msgVenue tdlib.MessageVenue
		json.Unmarshal(msgByte, &msgVenue)
	case tdlib.MessageContactType:
		var msgContact tdlib.MessageContact
		json.Unmarshal(msgByte, &msgContact)
	case tdlib.MessageGameType:
		var msgGame tdlib.MessageGame
		json.Unmarshal(msgByte, &msgGame)
	case tdlib.MessageInvoiceType:
		var msgInvoice tdlib.MessageInvoice
		json.Unmarshal(msgByte, &msgInvoice)
	case tdlib.MessageCallType:
		var msgCall tdlib.MessageCall
		json.Unmarshal(msgByte, &msgCall)
	case tdlib.MessageBasicGroupChatCreateType:
		var msgBasicGroupChatCreate tdlib.MessageBasicGroupChatCreate
		json.Unmarshal(msgByte, &msgBasicGroupChatCreate)
	case tdlib.MessageSupergroupChatCreateType:
		var msgSupergroupChatCreate tdlib.MessageSupergroupChatCreate
		json.Unmarshal(msgByte, &msgSupergroupChatCreate)
	case tdlib.MessageChatChangeTitleType:
		var msgChatChangeTitle tdlib.MessageChatChangeTitle
		json.Unmarshal(msgByte, &msgChatChangeTitle)
	case tdlib.MessageChatChangePhotoType:
		var msgChatChangePhoto tdlib.MessageChatChangePhoto
		json.Unmarshal(msgByte, &msgChatChangePhoto)
	case tdlib.MessageChatDeletePhotoType:
		var msgChatDeletePhoto tdlib.MessageChatDeletePhoto
		json.Unmarshal(msgByte, &msgChatDeletePhoto)
	case tdlib.MessageChatAddMembersType:
		var msgChatAddMembers tdlib.MessageChatAddMembers
		json.Unmarshal(msgByte, &msgChatAddMembers)
	case tdlib.MessageChatJoinByLinkType:
		var msgChatJoinByLink tdlib.MessageChatJoinByLink
		json.Unmarshal(msgByte, &msgChatJoinByLink)
	case tdlib.MessageChatDeleteMemberType:
		var msgChatDeleteMember tdlib.MessageChatDeleteMember
		json.Unmarshal(msgByte, &msgChatDeleteMember)
	case tdlib.MessageChatUpgradeToType:
		var msgChatUpgradeTo tdlib.MessageChatUpgradeTo
		json.Unmarshal(msgByte, &msgChatUpgradeTo)
	case tdlib.MessageChatUpgradeFromType:
		var msgChatUpgradeFrom tdlib.MessageChatUpgradeFrom
		json.Unmarshal(msgByte, &msgChatUpgradeFrom)
	case tdlib.MessagePinMessageType:
		var msgPinMessage tdlib.MessagePinMessage
		json.Unmarshal(msgByte, &msgPinMessage)
	case tdlib.MessageScreenshotTakenType:
		var msgScreenshotTaken tdlib.MessageScreenshotTaken
		json.Unmarshal(msgByte, &msgScreenshotTaken)
	case tdlib.MessageChatSetTTLType:
		var msgChatSetTTL tdlib.MessageChatSetTTL
		json.Unmarshal(msgByte, &msgChatSetTTL)
	case tdlib.MessageCustomServiceActionType:
		var msgCustomServiceAction tdlib.MessageCustomServiceAction
		json.Unmarshal(msgByte, &msgCustomServiceAction)
	case tdlib.MessageGameScoreType:
		var msgGameScore tdlib.MessageGameScore
		json.Unmarshal(msgByte, &msgGameScore)
	case tdlib.MessagePaymentSuccessfulType:
		var msgPaymentSuccessful tdlib.MessagePaymentSuccessful
		json.Unmarshal(msgByte, &msgPaymentSuccessful)
	case tdlib.MessagePaymentSuccessfulBotType:
		var msgPaymentSuccessfulBot tdlib.MessagePaymentSuccessfulBot
		json.Unmarshal(msgByte, &msgPaymentSuccessfulBot)
	case tdlib.MessageContactRegisteredType:
		var msgContactRegistered tdlib.MessageContactRegistered
		json.Unmarshal(msgByte, &msgContactRegistered)
	case tdlib.MessageWebsiteConnectedType:
		var msgWebsiteConnected tdlib.MessageWebsiteConnected
		json.Unmarshal(msgByte, &msgWebsiteConnected)
	case tdlib.MessagePassportDataSentType:
		var msgPassportDataSent tdlib.MessagePassportDataSent
		json.Unmarshal(msgByte, &msgPassportDataSent)
	case tdlib.MessagePassportDataReceivedType:
		var msgPassportDataReceived tdlib.MessagePassportDataReceived
		json.Unmarshal(msgByte, &msgPassportDataReceived)
	case tdlib.MessageUnsupportedType:
		var msgUnsupported tdlib.MessageUnsupported
		json.Unmarshal(msgByte, &msgUnsupported)

	default:
		fmt.Println("unknow type: ", msg.Content.GetMessageContentEnum())
	}
}

func saveFileById(fileid int32, priority int32) {
	fmt.Println("Create download task: %s, priority: %d", fileid, priority)

	tdf, err := client.DownloadFile(fileid, priority)
	if err != nil {
		fmt.Printf("Create download task error. %s \n", fileid)
		fmt.Println(err)
	}
	download = append(download, tdf)
	fmt.Printf("task: %d, file download path: %s \n", fileid, tdf.Local.Path)
}

func getRawMessage(client *tdlib.Client, limit int) {
	//rawUpdates gets all updates comming from tdlib
	rawUpdates := client.GetRawUpdatesChannel(100)

	for update := range rawUpdates {
		if update.Data["@type"].(string) == "error" {
			fmt.Printf("update message error! code: %d msg: %s \n", update.Data["code"], update.Data["message"])
			continue
		}

		tdtype := update.Data["@type"].(string)
		getUpdateMessageType(tdtype, update.Raw)

	}
}

func getUpdateMessageType(msgType string, msgByte []byte) {

	switch tdlib.UpdateEnum(msgType) {
	case tdlib.UpdateSupergroupType:
		var msgUpdateSupergroup tdlib.UpdateSupergroup
		err := json.Unmarshal(msgByte, &msgUpdateSupergroup)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("Super group: %d, Username: %s, Message Type: %s \n", msgUpdateSupergroup.Supergroup.ID, msgUpdateSupergroup.Supergroup.Username, msgUpdateSupergroup.Supergroup.MessageType())
	case tdlib.UpdateUserStatusType:
		// 更新用户是否在线
		var msgUpdateUserStatus tdlib.UpdateUserStatus
		err := json.Unmarshal(msgByte, &msgUpdateUserStatus)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("Update User: %d, status: %s \n", msgUpdateUserStatus.UserID, msgUpdateUserStatus.Status.GetUserStatusEnum())
	case tdlib.UpdateNewMessageType:
		var msgUpdateNewMessage tdlib.UpdateNewMessage
		err := json.Unmarshal(msgByte, &msgUpdateNewMessage)
		if err != nil {
			fmt.Println(err)
		}
		getMessageType(msgUpdateNewMessage.Message)
	case tdlib.UpdateChatLastMessageType:
		var msgUpdateChatLastMessage tdlib.UpdateChatLastMessage
		err := json.Unmarshal(msgByte, &msgUpdateChatLastMessage)
		if err != nil {
			fmt.Println(err)
		}
		getMessageType(msgUpdateChatLastMessage.LastMessage)
	case tdlib.UpdateChatReadInboxType:
		var msgUpdateChatReadInbox tdlib.UpdateChatReadInbox
		err := json.Unmarshal(msgByte, &msgUpdateChatReadInbox)
		if err != nil {
			fmt.Println(err)
		}
	case tdlib.UpdateChatReadOutboxType:
		var msgUpdateChatReadOutbox tdlib.UpdateChatReadOutbox
		err := json.Unmarshal(msgByte, &msgUpdateChatReadOutbox)
		if err != nil {
			fmt.Println(err)
		}

	case tdlib.UpdateUserType:
		var msgUpdateUser tdlib.UpdateUser
		err := json.Unmarshal(msgByte, &msgUpdateUser)
		if err != nil {
			fmt.Println(err)
		}

	case tdlib.UpdateSupergroupFullInfoType:
		var msgUpdateSupergroupFullinfo tdlib.UpdateSupergroupFullInfo
		err := json.Unmarshal(msgByte, &msgUpdateSupergroupFullinfo)
		if err != nil {
			fmt.Println(err)
		}

	case tdlib.UpdateUnreadMessageCountType:
		var msgUpdateUnreadMessageCount tdlib.UpdateUnreadMessageCount
		err := json.Unmarshal(msgByte, &msgUpdateUnreadMessageCount)
		if err != nil {
			fmt.Println(err)
		}

	default:
		fmt.Println(msgType, "not support.")
	}
}
