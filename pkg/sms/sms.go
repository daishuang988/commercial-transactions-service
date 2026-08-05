package sms

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
)

// Client 阿里云短信客户端
type Client struct {
	client   *dysmsapi.Client
	signName string
	template string
}

// NewClient 创建短信客户端
func NewClient(accessKeyID, accessKeySecret, signName, templateCode string) (*Client, error) {
	client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("创建短信客户端失败: %w", err)
	}
	return &Client{
		client:   client,
		signName: signName,
		template: templateCode,
	}, nil
}

// SendCode 发送验证码短信，返回生成的6位验证码
func (c *Client) SendCode(mobile string) (string, error) {
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	request := dysmsapi.CreateSendSmsRequest()
	request.Scheme = "https"
	request.PhoneNumbers = mobile
	request.SignName = c.signName
	request.TemplateCode = c.template
	request.TemplateParam = fmt.Sprintf(`{"code":"%s"}`, code)

	response, err := c.client.SendSms(request)
	if err != nil {
		return "", fmt.Errorf("发送短信失败: %w", err)
	}
	if response.Code != "OK" {
		return "", fmt.Errorf("短信发送失败: %s - %s", response.Code, response.Message)
	}
	return code, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
