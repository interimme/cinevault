package mailer

import (
	"bytes"
	"embed"
	"github.com/go-mail/mail/v2"
	"html/template"
	"time"
)

// The `templateFS` variable is an embedded file system (embed.FS) to hold email templates.
// The `//go:embed` directive is used to embed the contents of the "./templates" directory
// into the `templateFS` variable, allowing access to the email templates at runtime.
//
//go:embed "templates"
var templateFS embed.FS

// Mailer struct contains a mail.Dialer instance to connect to an SMTP server for sending emails,
// and a sender string to specify the "From" email address in the format "Name <email@example.com>".
type Mailer struct {
	dialer *mail.Dialer // SMTP dialer for sending emails.
	sender string       // Email address of the sender.
}

// New initializes and returns a new Mailer instance with the given SMTP server settings.
func New(host string, port int, username, password, sender string) Mailer {
	// Create a new mail.Dialer instance with the specified SMTP server settings (host, port, username, password).
	// The dialer is configured with a timeout of 5 seconds for sending emails.
	dialer := mail.NewDialer(host, port, username, password)
	dialer.Timeout = 5 * time.Second

	// Return a new Mailer instance containing the configured dialer and sender information.
	return Mailer{
		dialer: dialer,
		sender: sender,
	}
}

// Send composes and sends an email using the specified recipient, template file, and dynamic data.
// `recipient` is the email address to send to, `templateFile` is the filename of the email template,
// and `data` is dynamic content passed to the template for rendering.
func (m Mailer) Send(recipient, templateFile string, data interface{}) error {
	// Parse the email template from the embedded file system using the specified template file.
	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err // Return an error if parsing the template fails.
	}

	// Execute the "subject" template and store the result in a bytes.Buffer for the email subject.
	subject := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err // Return an error if executing the subject template fails.
	}

	// Execute the "plainBody" template and store the result in a bytes.Buffer for the plain-text email body.
	plainBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err // Return an error if executing the plain body template fails.
	}

	// Execute the "htmlBody" template and store the result in a bytes.Buffer for the HTML email body.
	htmlBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err // Return an error if executing the HTML body template fails.
	}

	// Create a new mail.Message instance and set the recipient, sender, and subject headers.
	// Set the plain-text body of the email using SetBody() and the HTML body using AddAlternative().
	// Note: AddAlternative() should always be called after SetBody() to properly set both content types.
	msg := mail.NewMessage()
	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", subject.String())
	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())

	// Send the email by calling DialAndSend() on the dialer with the message.
	// This method establishes a connection to the SMTP server, sends the email, and then closes the connection.
	// It returns an error if sending fails, such as a timeout or connection issue.
	err = m.dialer.DialAndSend(msg)
	if err != nil {
		return err // Return an error if sending the email fails.
	}

	return nil // Return nil if the email is sent successfully.
}
