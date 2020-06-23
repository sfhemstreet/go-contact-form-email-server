// Server used for recieving messages from website contact form.
// Sends reply email to messanger and forwards message to private email.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/cors"
	"html/template"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
)

// incomingMessage defines the structure of the message sent in the request body
type incomingMessage struct {
	Name  string
	Email string
	Title string
	Body  string
}

// Template for sending thank you email reply.
var thankYouEmailTemplate = template.Must(template.ParseFiles("htmlTemplates/thankYouEmail.html"))

func main() {
	// REMOVE BEFORE PUTTING ONLINE ------------------------------ IMPORTANT REMOVE BEFORE PUTTING ONLINE
	os.Setenv("PRIVATE_EMAIL", "sfhemstreet@gmail.com")
	os.Setenv("PUBLIC_EMAIL", "spencerhemstreet@gmail.com")
	os.Setenv("PUBLIC_EMAIL_PASSWORD", "vrklxekhwlaamqob")
	os.Setenv("ALLOWED_ORIGINS", "http://localhost:1234")

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		log.Fatal("ALLOWED_ORIGINS env variable not set")
	}

	allowedOriginsSlice := strings.Split(allowedOrigins, ",")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/contactFormEmail", handleMail)

	handler := cors.New(cors.Options{
		AllowedOrigins:   allowedOriginsSlice,
		AllowCredentials: true,
		AllowedMethods:   []string{"POST", "post"},
		Debug:            true,
	}).Handler(mux)

	log.Fatal(http.ListenAndServe(":8080", handler))
}

// handleMail sends a reply email to whoever sent me a message from my website,
// and also forwards the message they sent to my private email.
func handleMail(w http.ResponseWriter, r *http.Request) {

	// Public Email is the email I am using to send emails.
	// Public Email Password is used to set up the Auth for smtp.SendMail.
	// Private Email is the email I forward the inMsg to.
	publicEmail := os.Getenv("PUBLIC_EMAIL")
	publicEmailPassword := os.Getenv("PUBLIC_EMAIL_PASSWORD")
	privateEmail := os.Getenv("PRIVATE_EMAIL")
	if privateEmail == "" {
		log.Fatalln("Env variable PRIVATE_EMAIL is not set.")
	}
	if publicEmail == "" {
		log.Fatalln("Env variable PUBLIC_EMAIL is not set.")
	}
	if publicEmailPassword == "" {
		log.Fatalln("Env variable PUBLIC_EMAIL_PASSWORD is not set.")
	}

	// I need to get the incoming message from the request body.
	// This decodeJSON func insures the message is valid.
	var inMsg incomingMessage
	err := decodeJSONBody(w, r, &inMsg)
	if err != nil {
		var malReq *malformedRequest
		if errors.As(err, &malReq) {
			http.Error(w, malReq.msg, malReq.status)
		} else {
			log.Println(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	// Make messages that are going to be emailed.
	replyMsg := makeReplyEmail(inMsg, publicEmail)
	forwardMsg := makeForwardEmail(inMsg, privateEmail, publicEmail)

	// Auth and address for smtp service, I am using gmail.
	auth := smtp.PlainAuth("", publicEmail, publicEmailPassword, "smtp.gmail.com")
	addr := "smtp.gmail.com:587"

	// Send messages and check for errors.
	replyErr := smtp.SendMail(addr, auth, publicEmail, []string{inMsg.Email}, []byte(replyMsg))
	forwardErr := smtp.SendMail(addr, auth, publicEmail, []string{privateEmail}, []byte(forwardMsg))

	// Response object that the client expects back.
	response := struct{ Success bool }{Success: false}

	if forwardErr != nil {
		log.Printf("Forward message failed! Email: %s, Name: %s, Subject: %s, Body: %s", inMsg.Email, inMsg.Name, inMsg.Title, inMsg.Body)
		log.Println("Forward Error: ", forwardErr)
	}

	if replyErr != nil {
		log.Printf("Reply message failed! Email: %s, Name: %s, Subject: %s, Body: %s", inMsg.Email, inMsg.Name, inMsg.Title, inMsg.Body)
		log.Println("Reply Error: ", replyErr)

		response.Success = false
		responseJSON, err := json.Marshal(response)
		// If we fail to make JSON send an internal service error.
		if err != nil {
			http.Error(w, "Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJSON)
		return
	}

	response.Success = true
	responseJSON, err := json.Marshal(response)
	// If we fail to make JSON just send "Success"
	if err != nil {
		w.Write([]byte("Success"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

// makeReplyEmail creates an email with plain text and HTML to send
// to whoever messaged me in the first place. It says thanks and lets them know I got there message.
func makeReplyEmail(inMsg incomingMessage, fromEmail string) string {
	// Create HTML message to send in reply.
	// If this fails the email will be plain text only.
	replyData := struct{ Name string }{Name: inMsg.Name}
	htmlBuf := new(bytes.Buffer)
	err := thankYouEmailTemplate.Execute(htmlBuf, replyData)
	useHTML := err == nil
	if !useHTML {
		log.Println("Error parsing email template", err)
	}
	// Creates multipart MIME (plain text and HTML) that is very annoying and fragile.
	// CRLF or "\r\n" is very important and should not be messed with without double checking result.
	header := make(map[string]string)
	header["From"] = fmt.Sprintf("Spencer Hemstreet <%s>", fromEmail)
	header["To"] = fmt.Sprintf("%s <%s>", inMsg.Name, inMsg.Email)
	header["Subject"] = "You Contacted Spencer Hemstreet"
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "multipart/alternative; boundary=\"boundary123\""

	plainTextMsg := "--boundary123\nContent-Type: text/plain; charset=us-ascii\r\n"
	plainTextMsg += fmt.Sprintf("Hi %s,\n\nThank you for contacting me! I will get back to you soon.\n\nSincerely,\nSpencer Hemstreet\n", inMsg.Name)

	message := ""
	for key, value := range header {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	message += plainTextMsg

	if useHTML {
		htmlMsg := "--boundary123\nContent-Type: text/html\r\n" + htmlBuf.String()
		message += htmlMsg
	}
	// Insert ending boundary
	message += "\r\n--boundary123--"

	return message
}

// makeForwardEmail creates an email that is just plain text, with details about the message recieved from the client.
func makeForwardEmail(inMsg incomingMessage, toEmail string, fromEmail string) string {
	header := make(map[string]string)
	header["From"] = fmt.Sprintf("Spencer Hemstreet <%s>", fromEmail)
	header["To"] = fmt.Sprintf("Spencer <%s>", toEmail)
	header["Subject"] = fmt.Sprintf("Important: Contact Form Submission from %s", inMsg.Name)
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""

	body := fmt.Sprintf("\r\n%s at %s sent the following:\n\n%s\n\n%s\n\n", inMsg.Name, inMsg.Email, inMsg.Title, inMsg.Body)

	message := ""
	for key, value := range header {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	message += body

	return message
}
