package main

import (
	"log"
	"flag"

	"tf-chatbot/internal/chatbot"
)

func main() {
	var chatBot *chatbot.ChatBot
	var err error
	optConfig := flag.String("configPath", "/usr/local/etc/chatbotConfig.yaml", "ChatBot configuration path")
	optStaticPath := flag.String("staticPath", "/usr/local/etc/html", "ChatBot static HTML directory path")
	flag.Parse()
	if chatBot, err = chatbot.ChatBotInitialize(*optConfig, *optStaticPath); err != nil {
		log.Fatalf("failure to initialize ChatBot: %s", err)
	}
	chatBot.Run()
}
