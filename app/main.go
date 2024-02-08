package main

import(
	"net/http"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/boombuler/barcode"
    "github.com/boombuler/barcode/qr"
	"html/template"
	"io"
	"log"
	"gorm.io/gorm"
    "gorm.io/driver/sqlite"
	"os"
	"fmt"
	"image/png"
)

type TemplateRenderer struct{
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", name, err)
	}
	return err
}
var DB *gorm.DB

func Init() {
    var err error
    DB, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
    if err != nil {
        panic("failed to connect database")
    }
    // データベースのマイグレーション
    DB.AutoMigrate(&User{})
	DB.AutoMigrate(&UserInfoMation{})
}

type UserInfoMation struct {
    gorm.Model
    UserID        uint   // ユーザーテーブルの主キー
    PhoneNumber   string
    QRCODE_Number string
    Attend        bool
}

var e=createMux()



type User struct {
    gorm.Model
    UserName     string
    PhoneNumber  string
    UserInfo     UserInfoMation
}

func CreateUser(username, phoneNumber string) error {
    user := &User{UserName: username, PhoneNumber: phoneNumber}
    var userInfo UserInfoMation
    userInfo.PhoneNumber = user.PhoneNumber
    userInfo.QRCODE_Number = CreateQRCode(user.PhoneNumber)
    user.UserInfo = userInfo
    if err := DB.Create(user).Error; err != nil {
        return err
    }
    return nil
}

func main(){
	Init()

	e.Renderer=&TemplateRenderer{
		templates:template.Must(template.ParseGlob("../templates/*.html")),
	}

	e.GET("/main",func(c echo.Context)error{
		return c.Render(http.StatusOK,"index.html",nil)
	})


	// ログインフォームの表示用のルーティング
	e.GET("/login", func(c echo.Context) error {
		return c.Render(http.StatusOK, "login.html", nil)
	})

	e.Static("/css","../css")
	e.Static("/js","../js")
	e.Static("/images","../images")
	e.Static("/QRCode","../QRCode")

	// ログイン用のルーティング
	e.POST("/login", loginHandler)

	// ユーザー登録のルーティング
	e.POST("/register", registerHandler)
	port:=os.Getenv("PORT")
	if port==""{
		port="8080"
		log.Printf("DEfaulting to port %s",port)
	}
	log.Printf("Listening on port %s",port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s",port)))
}

func CreateQRCode(phoneNumber string) (string) {
    qrCode, _ := qr.Encode(phoneNumber, qr.M, qr.Auto)
    qrCode, _ = barcode.Scale(qrCode, 200, 200)
    fileName := fmt.Sprintf("../QRCode/%s_qrcode.png", phoneNumber)
    file, _ := os.Create(fileName)
    defer file.Close()

    png.Encode(file, qrCode)
    return phoneNumber + "_qrcode.png"
}


func createMux()*echo.Echo{
	e:=echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.Gzip())

	return e
}
func loginHandler(c echo.Context) error {
	username := c.FormValue("username")
	phoneNumber := c.FormValue("phoneNumber")

	var user User
	if err := DB.Where("user_name = ?", username).Preload("UserInfo").First(&user).Error; err != nil {
		return c.Render(http.StatusUnauthorized, "login.html", map[string]interface{}{
			"Error": "Invalid credentials",
		})
	}
	// パスワードのハッシュ化はセキュリティ上の理由から必要です
	// ここでは簡単な例として平文のパスワードをそのまま比較します
	if user.PhoneNumber == phoneNumber {
		return c.Render(http.StatusOK, "login_success.html", map[string]interface{}{
			"Username": username,
			"img":user.UserInfo.QRCODE_Number,
		})
	}

	return c.Render(http.StatusUnauthorized, "login.html", map[string]interface{}{
		"Error": "Invalid username or phone number",
	})
}

func registerHandler(c echo.Context) error {
    username := c.FormValue("username")
    password := c.FormValue("phoneNumber")

    if err := CreateUser(username, password); err != nil {
        return c.String(http.StatusInternalServerError, "Failed to create user")
    }

    return c.String(http.StatusOK, "User registered successfully")
}

