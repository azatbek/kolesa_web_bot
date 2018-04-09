package bot

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"fmt"
	"strconv"
	"strings"
	"../config"
	"../db"
	"time"
)

var (
	index     int
	asked     bool
	started   bool
	lastQId   int
	questions []db.Questions
	quiz      Quiz
	logs      []Log
	variants  []string
)

type BotApi struct {
	BotApi  *tgbotapi.BotAPI
	Updates tgbotapi.UpdatesChannel
	Update  tgbotapi.Update
}

type Quiz struct {
	User      string
	Score     int
	StartTime int64
	EndTime   int64
	Log       []Log
}

type Log struct {
	QuestionId int
	AnswerId   int
}

func (bot *BotApi) ListenForUpdates()  {
	variants = []string{"A) ", "B) ", "C) ", "D) "}

	for update := range bot.Updates {
		bot.Update = update

		if update.Message != nil {
			bot.messageUpdateListener()
		} else {
			if update.CallbackQuery != nil {
				bot.callbackQueryListener()
			}
		}
	}
}

func (bot *BotApi) messageUpdateListener()  {
	if bot.Update.Message.Command() == "" {
		bot.messageListener()
	} else {
		bot.commandListener()
	}
}

func (bot *BotApi) callbackQueryListener()  {
	bot.dynamicCallbackQuery()

	switch bot.Update.CallbackQuery.Data {
	case "schedule":
		schedule := getSchedule()

		msg := newMessage(
			bot.Update.CallbackQuery.Message.Chat.ID,
			getText("schedule") + "\n\n" + schedule.Value,
			"html")

		bot.BotApi.Send(msg)
	case "ask":
		msg := newMessage(
			bot.Update.CallbackQuery.Message.Chat.ID,
			getText("ask"),
			"html")

		bot.BotApi.Send(msg)

		asked = true
	case "faq":
		msg := newMessage(bot.Update.CallbackQuery.Message.Chat.ID, getText("faq"), "html")

		keyboard := tgbotapi.InlineKeyboardMarkup{}
		questions := getFaq()


		for _, item := range questions {
			var row []tgbotapi.InlineKeyboardButton
			btn := tgbotapi.NewInlineKeyboardButtonData(item.Question, "faq_" + strconv.Itoa(item.Id))
			row = append(row, btn)
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
		}

		msg.ReplyMarkup = keyboard
		bot.BotApi.Send(msg)
	case "test":
		msg := newMessage(
			bot.Update.CallbackQuery.Message.Chat.ID,
			getText("test"),
			"html")

		keyboard := tgbotapi.InlineKeyboardMarkup{}

		var row []tgbotapi.InlineKeyboardButton
		btn := tgbotapi.NewInlineKeyboardButtonData(getText("startTest") + " " + getEmoji("right-arrow"), "startTest")
		row = append(row, btn)
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)

		msg.ReplyMarkup = keyboard
		bot.BotApi.Send(msg)
	case "startTest":
		bot.BotApi.DeleteMessage(tgbotapi.DeleteMessageConfig{bot.Update.CallbackQuery.Message.Chat.ID, bot.Update.CallbackQuery.Message.MessageID})

		if started {
			bot.BotApi.DeleteMessage(tgbotapi.DeleteMessageConfig{bot.Update.CallbackQuery.Message.Chat.ID, lastQId})
		}

		if checkIfUserExists(bot.Update.CallbackQuery.Message.Chat.UserName) {
			msg := newMessage(
				bot.Update.CallbackQuery.Message.Chat.ID,
				getText("recorded"),
				"html")

			bot.BotApi.Send(msg)
		} else {
			index = 0
			questions = getRandQuestions()
			started = true

			bot.newQuestionMessage(bot.Update.CallbackQuery.Message.Chat.ID)
		}
	}
}

func (bot *BotApi) messageListener() {
	if started {
		warningMessage := newMessage(
			bot.Update.Message.Chat.ID,
			getText("continueTest"),
			"html")

		bot.BotApi.Send(warningMessage)
	}

	if asked && bot.Update.Message.Text != "" {
		channelId, err := strconv.ParseInt(config.Toml.Bot.ChannelId, 10, 64);

		if err != nil {
			fmt.Println(err)
		}

		msg := tgbotapi.NewForward(channelId, bot.Update.Message.Chat.ID, bot.Update.Message.MessageID)

		bot.BotApi.Send(msg)

		confirmMsg := newMessage(
			bot.Update.Message.Chat.ID,
			getText("confirmed"),
			"html")

		bot.BotApi.Send(confirmMsg)

		asked = false
	}
}

func (bot *BotApi) commandListener()  {
	switch bot.Update.Message.Command() {
	case "start", "menu":
		msg := newMessage(bot.Update.Message.Chat.ID, "<b>Меню:</b>", "html")

		keyboard := tgbotapi.InlineKeyboardMarkup{}
		menu := getMenu()

		for i, item := range menu {
			var row []tgbotapi.InlineKeyboardButton
			btn := tgbotapi.NewInlineKeyboardButtonData(item.Name + " " + menuEmojiList()[i], item.Alias)
			row = append(row, btn)
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
		}

		msg.ReplyMarkup = keyboard
		bot.BotApi.Send(msg)
	}
}

func (bot *BotApi) dynamicCallbackQuery()  {
	if strings.Contains(bot.Update.CallbackQuery.Data, "faq_") {
		bot.faqCallbackQuery()
	} else if strings.Contains(bot.Update.CallbackQuery.Data, "variant_") {
		bot.variantCallbackQuery()
	}
}

func (bot *BotApi) faqCallbackQuery()  {
	s := strings.Split(bot.Update.CallbackQuery.Data, "_")
	id, err := strconv.Atoi(s[1])

	if err != nil {
		fmt.Println(err)
	}

	question := getQuestion(id)

	msg := newMessage(
		bot.Update.CallbackQuery.Message.Chat.ID,
		"<b>" + question.Question + "</b>" + "\n\n" + question.Answer,
		"html")

	bot.BotApi.Send(msg)
}

func (bot *BotApi) variantCallbackQuery() {
	callBackQuery := bot.Update.CallbackQuery
	s := strings.Split(bot.Update.CallbackQuery.Data, "_")
	i, err := strconv.Atoi(s[1])
	id, err := strconv.Atoi(s[2])

	if err != nil {
		fmt.Println(err)
	}

	logs = append(logs, Log{QuestionId: questions[index].Id, AnswerId: id})

	if index == 0 {
		quiz = Quiz{
			User: callBackQuery.Message.Chat.UserName,
			Score: questions[index].Variants[i].Value,
			StartTime: time.Now().Unix(),
		}

		index++
		bot.newQuestionMessage(callBackQuery.Message.Chat.ID)
	} else if index == 5 {
		quiz.Log = logs
		quiz.Score += questions[index].Variants[i].Value
		quiz.EndTime = time.Now().Unix()

		scoreStr := strconv.Itoa(quiz.Score)

		if err != nil {
			fmt.Println(err)
		}

		started = false
		newQuizRecord(quiz)

		msg := newMessage(
			callBackQuery.Message.Chat.ID,
			getText("score") + scoreStr + "</b>",
			"html")

		bot.BotApi.Send(msg)
	} else {
		quiz.Score += questions[index].Variants[i].Value

		index++
		bot.newQuestionMessage(callBackQuery.Message.Chat.ID)
	}

	bot.BotApi.DeleteMessage(tgbotapi.DeleteMessageConfig{callBackQuery.Message.Chat.ID, callBackQuery.Message.MessageID})
}

func (bot *BotApi) newQuestionMessage(chatId int64) {
	var err error
	indexStr := strconv.Itoa(index + 1)
	msg := newMessage(chatId, "<b>" + indexStr + ")</b> " + questions[index].Text, "html")

	keyboard := tgbotapi.InlineKeyboardMarkup{}

	for i, item := range questions[index].Variants {
		var row []tgbotapi.InlineKeyboardButton
		btn := tgbotapi.NewInlineKeyboardButtonData(variants[i] + item.Text, "variant_" + strconv.Itoa(i) + "_" + strconv.Itoa(item.Id))
		row = append(row, btn)
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
	}

	msg.ReplyMarkup = keyboard
	message, err := bot.BotApi.Send(msg)

	if err != nil {
		fmt.Println(err)
	}

	lastQId = message.MessageID
}

func newMessage(chatId int64, text string, parseMode string) tgbotapi.MessageConfig {
	return tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:           chatId,
			ReplyToMessageID: 0,
		},
		Text: text,
		ParseMode: parseMode,
		DisableWebPagePreview: false,
	}
}