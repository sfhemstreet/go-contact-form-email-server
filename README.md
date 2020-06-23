# Go Contact Form Email Server

Server that receives messages from my personal website's contact form, 
sends out a reply email to whoever messaged me, and forwards the message to my personal email account.

## Why
 
I wanted to learn Go, and I also wanted to ditch my old email solution that relied on SendGrid.  Hopefully this helps others with building a similar, simple, free email solution.

## How

server.go creates an [HTTP request multiplexer](https://golang.org/pkg/net/http/#ServeMux) that listens for requests to /email.  When a contact form is submitted it hits this endpoint.  The contact form is decoded and validated by decodeJSON.go, and the reply and forward emails are made using the html/template from the Go standard library.  Then using net/smtp, sends the emails using gmails smtp server.  (Gmail accounts can use gmail's smtp server for free, all you need to do is set up an app password)


