package notifications

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"

	"path/filepath"

	"github.com/grafana/grafana/pkg/bus"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

var mailTemplates *template.Template

func Init() error {
	initMailQueue()

	bus.AddHandler("email", sendEmailCommandHandler)

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

func sendEmailCommandHandler(cmd *m.SendEmailCommand) error {
	if !setting.Smtp.Enabled {
		return errors.New("Grafana mailing/smtp options not configured, contact your Grafana admin")
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
