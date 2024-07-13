package main

import (
	"fmt"
	"html/template"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type TemplateRenderer struct {
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
	UserID        uint // ユーザーテーブルの主キー
	PhoneNumber   string
	QRCODE_Number string
	Attend        bool
}

var e = createMux()

var ScanUserData User

type User struct {
	gorm.Model
	//新郎側か新婦側か
	//新郎側がtrue
	MenOrWomen bool
	//氏名
	UserName string
	//電話番号
	PhoneNumber string
	// アレルギー情報
	AllergyInfo string
	// 同伴者
	Companion int
	// Userが入力しない情報
	UserInfo UserInfoMation
}

func main() {
	Init()

	e.Renderer = &TemplateRenderer{
		templates: template.Must(template.ParseGlob("../templates/*.html")),
	}

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})

	// ログインフォームの表示用のルーティング
	e.GET("/login", func(c echo.Context) error {
		return c.Render(http.StatusOK, "login.html", nil)
	})

	e.GET("/signup", func(c echo.Context) error {
		return c.Render(http.StatusOK, "signup.html", nil)
	})
	e.GET("/qrcode", func(c echo.Context) error {
		return c.Render(http.StatusOK, "QRCodeRead.html", nil)
	})
	
	e.GET("/dblogin",func(c echo.Context)error{
		return c.Render(http.StatusOK,"dblogin.html",nil)
	})


	e.Static("/css", "../css")
	e.Static("/js", "../js")
	e.Static("/images", "../images")
	e.Static("/QRCode", "../QRCode")

	// ログイン用のルーティング
	e.POST("/login", loginHandler)

	// ユーザー登録のルーティング
	e.POST("/signup", signupHandler)

	e.POST("/db",DbConnectionHandler)

	e.POST("/scanResult",QRScanHandler)

	e.POST("/confirmation",ConfirmationHandler)

	e.POST("/infoUpDate",InfoUpDateHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("DEfaulting to port %s", port)
	}


	log.Printf("Listening on port %s", port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}

func CreateQRCode(phoneNumber string) string {
	qrCode, _ := qr.Encode(phoneNumber, qr.M, qr.Auto)
	qrCode, _ = barcode.Scale(qrCode, 200, 200)
	fileName := fmt.Sprintf("../QRCode/%s_qrcode.png", phoneNumber)
	file, _ := os.Create(fileName)
	defer file.Close()

	png.Encode(file, qrCode)
	return phoneNumber + "_qrcode.png"
}

func createMux() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.Gzip())

	return e
}
func loginHandler(c echo.Context) error {
	phoneNumber := c.FormValue("phoneNumber")
	var user User
	if err := DB.Where("phone_number = ?", phoneNumber).Preload("UserInfo").First(&user).Error; err != nil {
		return c.Render(http.StatusUnauthorized, "login.html", map[string]interface{}{
			"Error": "Invalid credentials",
		})
	}
	if user.PhoneNumber == phoneNumber {
		return c.Render(http.StatusOK, "login_success.html", map[string]interface{}{
			"UserName": user.UserName,
			"Img":      user.UserInfo.QRCODE_Number,
			"MenOrWomenInfo":user.MenOrWomen,
			"PhoneNumber":user.PhoneNumber,
			"AllergyInfo":user.AllergyInfo,
			"Companion":user.Companion,
		})
	}

	return c.Render(http.StatusUnauthorized, "login.html", map[string]interface{}{
		"Error": "Invalid username or phone number",
	})
}

func signupHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("phoneNumber")
	allergyInfo:=c.FormValue("allergyInfo")
	companion:=c.FormValue("companion")
	morw:=c.FormValue("morw")
	var menOrwoman bool
	if morw!=""{
		menOrwoman=true
	}
	
	_,err := CreateUser(username, password,allergyInfo,companion,menOrwoman)
	if err!=nil{
		return c.String(http.StatusInternalServerError, "Failed to create user")
	}
	return c.Render(http.StatusOK, "index.html",nil)
}

func CreateUser(username, phoneNumber,allergyInfo,companion string,menOrwoman bool)(*User,error){
	numCompanion,_:=strconv.Atoi(companion)
	user := &User{UserName: username, PhoneNumber: phoneNumber,AllergyInfo:allergyInfo,Companion:numCompanion,MenOrWomen:menOrwoman}
	var userInfo UserInfoMation
	userInfo.PhoneNumber = user.PhoneNumber
	userInfo.QRCODE_Number = CreateQRCode(user.PhoneNumber)
	user.UserInfo = userInfo
	if err := DB.Create(user).Error; err != nil {
		return nil,err
	}
	return user,nil
}


func QRScanHandler(c echo.Context) error {
	// QRコードデータを取得
	qrCodeData := c.FormValue("qrCodeData")

	var user User

	// データベースからQRコードデータに一致するphoneNumberを持つUserを検索
	if err := DB.Where("phone_number = ?", qrCodeData).Preload("UserInfo").First(&user).Error; err != nil {
		// 一致するUserが見つからない場合の処理
		if err == gorm.ErrRecordNotFound {
			// ユーザーが見つからないエラーメッセージを返す
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		// その他のエラー
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}
	
	ScanUserData=user

	// 成功した場合、クライアントに対して成功のメッセージを返す
	return c.Render(http.StatusOK,"confirmation.html",map[string]interface{}{
		"UserName":user.UserName,
	})
}


func DbConnectionHandler(c echo.Context)error{
	passWord:=c.FormValue("password")
	if(passWord=="kento1201"){
		return DbOpenHandler(c)
	}
	return nil
}


	func DbOpenHandler(c echo.Context) error {
		var users []User
		var userInfos []UserInfoMation
	
		// ユーザーデータを取得
		if err := DB.Find(&users).Error; err != nil {
			log.Printf("Failed to get users: %v", err)
		}
	
		// ユーザー情報を取得
		if err := DB.Find(&userInfos).Error; err != nil {
			log.Printf("Failed to get user infos: %v", err)
		}
		return c.Render(http.StatusOK, "dbhome.html", map[string]interface{}{
			"Users":      users,
			"UserInfos":  userInfos,
		})
	}


func ConfirmationHandler(c echo.Context)error{
    // UserInfoのAttendフィールドをtrueに更新
    ScanUserData.UserInfo.Attend = true
    if err := DB.Save(&ScanUserData.UserInfo).Error; err != nil {
        // データベース更新時のエラー処理
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user info"})
    }
	userName:=ScanUserData.UserName
	var user User
	ScanUserData=user
    return c.Render(http.StatusOK,"confirmationTrue.html",map[string]interface{}{
		"UserName":userName,
	})
}


func InfoUpDateHandler(c echo.Context)error{
	mOrW:=c.FormValue("morw")
	userName:=c.FormValue("username")
	phoneNumber:=c.FormValue("phoneNumber")
	allergyInfo:=c.FormValue("allergyInfo")
	companion:=c.FormValue("companion")
	var upDateUser User
	if mOrW=="新郎側"{
		upDateUser.MenOrWomen=true
	}else{
		upDateUser.MenOrWomen=false
	}
	upDateUser.UserName=userName
	upDateUser.PhoneNumber=phoneNumber
	upDateUser.AllergyInfo=allergyInfo
	num,err:=strconv.Atoi(companion)
	if err!=nil{
		return err
	}
	upDateUser.Companion=num
	return err
}