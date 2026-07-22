package front

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"commercial-transactions-service/internal/config"
	"commercial-transactions-service/internal/middleware"
	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"
	"commercial-transactions-service/pkg/utils"

	"github.com/gin-gonic/gin"
)

var Cfg *config.Config

func Init(cfg *config.Config) { Cfg = cfg }

// Login 用户登录 POST /api/v1/front/auth/login
func Login(c *gin.Context) {
	var req model.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请输入账号和密码")
		return
	}
	u, err := repository.GetUserByUsername(req.Username)
	if err != nil || u.Status == 0 {
		app.Fail(c, app.ErrCodeUserNotFound, "账号不存在或已冻结")
		return
	}
	// 支持两种密码格式：老系统 MD5(密码+盐) 或新系统 bcrypt
	valid := false
	if len(u.Password) > 4 && u.Password[:4] == "$2a$" {
		valid = utils.CheckPassword(req.Password, u.Password)
	} else {
		valid = utils.MD5Hash(req.Password+u.Salt) == u.Password
	}
	if !valid {
		app.Fail(c, app.ErrCodePasswordWrong, "密码错误")
		return
	}
	token, err := middleware.GenerateToken(u.ID, u.Username, false, Cfg.JWT.ExpireHours)
	if err != nil {
		app.InternalError(c, "生成Token失败")
		return
	}
	app.OK(c, gin.H{
		"token":    token,
		"user_id":  u.ID,
		"username": u.Username,
		"nickname": u.Nickname,
	})
}

// Register 用户注册 POST /api/v1/front/auth/register
func Register(c *gin.Context) {
	var req model.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请输入完整信息")
		return
	}
	if _, err := repository.GetUserByUsername(req.Username); err == nil {
		app.Fail(c, app.ErrCodeUserExists, "账号已存在")
		return
	}
	salt := utils.RandStr(6)
	invite := utils.RandStr(6)
	for {
		if _, err := repository.GetUserByInviteCode(invite); err != nil {
			break
		}
		invite = utils.RandStr(6)
	}
	now := time.Now()
	u := &model.User{
		Username:  req.Username,
		Nickname:  req.Nickname,
		Mobile:    req.Mobile,
		Password:  utils.MD5Hash(req.Password + salt), // 兼容老格式
		Salt:      salt,
		Sex:       0,
		Avatar:    "/assets/img/avatar.png",
		Invite:    invite,
		Level:     1,
		Status:    1,
		PID:       0,
		JoinTime:  &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if req.InviteCode != "" {
		if parent, err := repository.GetUserByInviteCode(req.InviteCode); err == nil {
			u.PID = parent.ID
		}
	}
	if err := repository.CreateUser(u); err != nil {
		app.InternalError(c, "注册失败")
		return
	}
	// 自动创建钱包
	repository.DB.Create(&model.UserWallet{
		UserID: u.ID, Money: 0, Coupon: 0,
		SelfBonus: 0, ShareBonus: 0, Score: 0, Poor: 0,
		UpdatedAt: now,
	})
	app.OK(c, gin.H{"user_id": u.ID, "username": u.Username})
}

// ─── 验证码 & 短信 ───

var (
	captchaStore = sync.Map{} // captcha_id → answer
	smsStore     = sync.Map{} // mobile → {code, expire}
)

type smsRecord struct {
	Code   string
	Expire time.Time
}

// Captcha 获取图形验证码 GET /api/v1/front/captcha
func Captcha(c *gin.Context) {
	a := time.Now().UnixNano()%20 + 1
	b := time.Now().UnixNano()%20 + 1
	answer := strconv.Itoa(int(a) + int(b))
	captchaID := utils.RandStr(16)
	captchaStore.Store(captchaID, answer)

	// 清理过期验证码(超过5分钟)
	go func() {
		time.Sleep(5 * time.Minute)
		captchaStore.Delete(captchaID)
	}()

	app.OK(c, gin.H{
		"captcha_id": captchaID,
		"question":   fmt.Sprintf("%d + %d = ?", a, b),
		"expire":     "5分钟",
	})
}

// SendSMS 发送短信验证码 POST /api/v1/front/sms/send
func SendSMS(c *gin.Context) {
	var req struct {
		Mobile      string `json:"mobile" binding:"required"`
		CaptchaID   string `json:"captcha_id" binding:"required"`
		CaptchaCode string `json:"captcha_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}

	// 验证图形验证码
	ans, ok := captchaStore.LoadAndDelete(req.CaptchaID)
	if !ok || ans.(string) != req.CaptchaCode {
		app.BadRequest(c, "图形验证码错误或已过期")
		return
	}

	// 检查60秒内是否已发送
	if v, ok := smsStore.Load(req.Mobile); ok {
		r := v.(smsRecord)
		if time.Now().Before(r.Expire.Add(-4 * time.Minute)) {
			app.TooManyRequests(c, "60秒内已发送，请稍后再试")
			return
		}
	}

	// 生成6位数字验证码
	code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	smsStore.Store(req.Mobile, smsRecord{Code: code, Expire: time.Now().Add(5 * time.Minute)})

	// TODO: 接入真实短信网关，生产环境去掉 code 返回
	fmt.Printf("[SMS] 手机号=%s 验证码=%s\n", req.Mobile, code)

	app.OK(c, gin.H{"msg": "验证码已发送", "code": code, "expire": "5分钟"})
}

// RegisterReqV2 注册请求
type RegisterReqV2 struct {
	Mobile     string `json:"mobile" binding:"required"`
	Password   string `json:"password" binding:"required,min=6"`
	SmsCode    string `json:"sms_code" binding:"required"`
	InviteCode string `json:"invite_code" binding:"required"`
	Nickname   string `json:"nickname"`
}

// RegisterV2 手机验证码注册 POST /api/v1/front/auth/register-v2
func RegisterV2(c *gin.Context) {
	var req RegisterReqV2
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请填写完整信息（手机号、密码、验证码、邀请码为必填）")
		return
	}

	// 验证短信验证码（Mock: 暂写死 1234）
	if req.SmsCode != "1234" {
		app.BadRequest(c, "验证码错误")
		return
	}

	// 检查手机号是否已注册
	if _, err := repository.GetUserByUsername(req.Mobile); err == nil {
		app.Fail(c, app.ErrCodeUserExists, "该手机号已注册，请直接登录")
		return
	}

	// 校验邀请码是否存在
	parent, err := repository.GetUserByInviteCode(req.InviteCode)
	if err != nil || parent == nil {
		app.BadRequest(c, "邀请码无效，请检查后重新输入")
		return
	}

	salt := utils.RandStr(6)
	myInvite := utils.RandStr(6)
	for {
		if _, err := repository.GetUserByInviteCode(myInvite); err != nil {
			break
		}
		myInvite = utils.RandStr(6)
	}
	now := time.Now()
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Mobile
	}
	u := &model.User{
		Username:  req.Mobile,
		Nickname:  nickname,
		Mobile:    req.Mobile,
		Password:  utils.MD5Hash(req.Password + salt),
		Salt:      salt,
		Sex:       0,
		Avatar:    "/assets/img/avatar.png",
		Invite:    myInvite,
		Level:     1,
		Status:    1,
		PID:       parent.ID,
		JoinTime:  &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repository.CreateUser(u); err != nil {
		app.InternalError(c, "注册失败")
		return
	}
	repository.DB.Create(&model.UserWallet{
		UserID: u.ID, Money: 0, Coupon: 0,
		SelfBonus: 0, ShareBonus: 0, Score: 0, Poor: 0,
		UpdatedAt: now,
	})
	app.OK(c, gin.H{"user_id": u.ID, "mobile": u.Mobile})
}

// ResetPassword 短信重置密码 POST /api/v1/front/auth/reset-password
func ResetPassword(c *gin.Context) {
	var req struct {
		Mobile      string `json:"mobile" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
		SmsCode     string `json:"sms_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}

	// 验证短信验证码（Mock: 暂写死 1234）
	if req.SmsCode != "1234" {
		app.BadRequest(c, "验证码错误")
		return
	}

	// 查找用户
	u, err := repository.GetUserByUsername(req.Mobile)
	if err != nil || u == nil {
		app.NotFound(c, "该手机号未注册")
		return
	}

	// 更新密码（兼容老格式 MD5+盐）
	salt := utils.RandStr(6)
	repository.DB.Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]interface{}{
		"password":   utils.MD5Hash(req.NewPassword + salt),
		"salt":       salt,
		"updated_at": time.Now(),
	})

	app.OK(c, gin.H{"msg": "密码重置成功"})
}

// Profile 个人信息 GET /api/v1/front/user/profile
func Profile(c *gin.Context) {
	uid := c.GetInt64("user_id")
	u, _ := repository.GetUserByID(uid)
	w, _ := repository.GetUserWallet(uid)
	if u == nil {
		app.NotFound(c, "用户不存在")
		return
	}
	app.OK(c, gin.H{
		"user":   u,
		"wallet": w,
	})
}

// Wallet 钱包 GET /api/v1/front/user/wallet
func Wallet(c *gin.Context) {
	uid := c.GetInt64("user_id")
	w, err := repository.GetUserWallet(uid)
	if err != nil {
		app.NotFound(c, "钱包不存在")
		return
	}
	app.OK(c, w)
}

// ChangePassword 修改密码 PUT /api/v1/front/user/password
func ChangePassword(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误，新密码至少6位")
		return
	}
	if req.OldPassword == req.NewPassword {
		app.BadRequest(c, "新密码不能与原密码相同")
		return
	}

	// 查用户当前密码
	var u model.User
	if err := repository.DB.Where("id = ?", uid).First(&u).Error; err != nil {
		app.NotFound(c, "用户不存在")
		return
	}

	// 校验原密码（兼容 MD5 和 bcrypt）
	valid := false
	if len(u.Password) > 4 && u.Password[:4] == "$2a$" {
		valid = utils.CheckPassword(req.OldPassword, u.Password)
	} else {
		valid = utils.MD5Hash(req.OldPassword+u.Salt) == u.Password
	}
	if !valid {
		app.Fail(c, app.ErrCodeOldPasswordWrong, "原密码错误")
		return
	}

	// 更新密码（保持与现有格式一致：MD5+盐）
	salt := utils.RandStr(6)
	repository.DB.Model(&model.User{}).Where("id = ?", uid).Updates(map[string]interface{}{
		"password":   utils.MD5Hash(req.NewPassword + salt),
		"salt":       salt,
		"updated_at": time.Now(),
	})

	app.OK(c, gin.H{"msg": "密码修改成功"})
}
