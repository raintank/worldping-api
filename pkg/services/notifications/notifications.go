package notifications

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"

	"path/filepath"

	"github.com/grafana/grafana/pkg/util"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/setting"
)

var mailTemplates *template.Template

func Init() error {
	initMailQueue()

	mailTemplates = template.New("name")
	mailTemplates.Funcs(template.FuncMap{
		"Subject": subjectTemplateFunc,
	})

	templatePattern := filepath.Join(setting.StaticRootPath, setting.Smtp.TemplatesPattern)
	_, err := mailTemplates.ParseGlob(templatePattern)
	if err != nil {
		return err
	}

	if !util.IsEmail(setting.Smtp.FromAddress) {
		return errors.New("Invalid email address for smpt from_adress config")
	}

	return nil
}

func subjectTemplateFunc(obj map[string]interface{}, value string) string {
	obj["value"] = value
	return ""
}

func SendEmail(cmd *m.SendEmailCommand) error {
	if !setting.Smtp.Enabled {
		return errors.New("Worldping mailing/smtp options not configured, contact your network admin")
	}
	if mailTemplates == nil {
		log.Fatal(4, "email templates not yet initialized.")
	}

	var buffer bytes.Buffer
	var err error
	var subjectText interface{}

	data := cmd.Data
	if data == nil {
		data = make(map[string]interface{}, 10)
	}

	setDefaultTemplateData(data)
	err = mailTemplates.ExecuteTemplate(&buffer, cmd.Template, data)
	if err != nil {
		return err
	}

	subjectData := data["Subject"].(map[string]interface{})
	subjectText, hasSubject := subjectData["value"]

	if !hasSubject {
		return errors.New(fmt.Sprintf("Missing subject in Template %s", cmd.Template))
	}

	subjectTmpl, err := template.New("subject").Parse(subjectText.(string))
	if err != nil {
		return err
	}

	var subjectBuffer bytes.Buffer
	err = subjectTmpl.ExecuteTemplate(&subjectBuffer, "subject", data)
	if err != nil {
		return err
	}

	addToMailQueue(&Message{
		To:      cmd.To,
		From:    setting.Smtp.FromAddress,
		Subject: subjectBuffer.String(),
		Body:    buffer.String(),
	})

	return nil
}
