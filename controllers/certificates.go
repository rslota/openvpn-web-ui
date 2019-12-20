package controllers

import (
	"archive/zip"
	"fmt"
	"github.com/adamwalach/openvpn-web-ui/state"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/adamwalach/go-openvpn/client/config"
	"github.com/adamwalach/openvpn-web-ui/lib"
	"github.com/adamwalach/openvpn-web-ui/models"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/validation"
	"io/ioutil"
)

type NewCertParams struct {
	Name string `form:"Name" valid:"Required;"`
}

type CertificatesController struct {
	BaseController
	ConfigDir string
}

func (c *CertificatesController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}
	c.Data["breadcrumbs"] = &BreadCrumbs{
		Title: "Certificates",
	}
}

// @router /certificates/:key [get]
func (c *CertificatesController) Download() {
	name := c.GetString(":key")
	filename := fmt.Sprintf("%s.ovpn", name)

	c.Ctx.Output.Header("Content-Type", "application/octet-stream")
	c.Ctx.Output.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	keysPath := filepath.Join(state.GlobalCfg.OVConfigPath, "keys")
	cfgPath, err := c.saveClientConfig(keysPath, name)
	if err == nil {
		beego.Error(err)
		return
	}
	data, err := ioutil.ReadFile(cfgPath)
	if err == nil {
		beego.Error(err)
		return
	}
	if _, err = c.Controller.Ctx.ResponseWriter.Write(data); err == nil {
		beego.Error(err)
	}
}

func addFileToZip(zw *zip.Writer, path string) error {
	header := &zip.FileHeader{
		Name:     filepath.Base(path),
		Method:   zip.Store,
		Modified: time.Now(),
	}
	fi, err := os.Open(path)
	if err != nil {
		beego.Error(err)
		return err
	}

	fw, err := zw.CreateHeader(header)
	if err != nil {
		beego.Error(err)
		return err
	}

	if _, err = io.Copy(fw, fi); err != nil {
		beego.Error(err)
		return err
	}

	return fi.Close()
}

// @router /certificates [get]
func (c *CertificatesController) Get() {
	c.TplName = "certificates.html"
	c.showCerts()
}

func (c *CertificatesController) showCerts() {
	path := filepath.Join(state.GlobalCfg.OVConfigPath, "keys/index.txt")
	certs, err := lib.ReadCerts(path)
	if err != nil {
		beego.Error(err)
	}
	lib.Dump(certs)
	c.Data["certificates"] = &certs
}

// @router /certificates [post]
func (c *CertificatesController) Post() {
	c.TplName = "certificates.html"
	flash := beego.NewFlash()

	cParams := NewCertParams{}
	if err := c.ParseForm(&cParams); err != nil {
		beego.Error(err)
		flash.Error(err.Error())
		flash.Store(&c.Controller)
	} else {
		if vMap := validateCertParams(cParams); vMap != nil {
			c.Data["validation"] = vMap
		} else {
			if err := lib.CreateCertificate(cParams.Name); err != nil {
				beego.Error(err)
				flash.Error(err.Error())
				flash.Store(&c.Controller)
			}
		}
	}
	c.showCerts()
}

func validateCertParams(cert NewCertParams) map[string]map[string]string {
	valid := validation.Validation{}
	b, err := valid.Valid(&cert)
	if err != nil {
		beego.Error(err)
		return nil
	}
	if !b {
		return lib.CreateValidationMap(valid)
	}
	return nil
}

func (c *CertificatesController) saveClientConfig(keysPath string, name string) (string, error) {
	cfg := config.New()
	cfg.ServerAddress = state.GlobalCfg.ServerAddress
	ca, err := ioutil.ReadFile(filepath.Join(keysPath, "ca.crt"))
	if err != nil {
		return "", err
	}
	cfg.Ca = string(ca)
	cert, err := ioutil.ReadFile(filepath.Join(keysPath, name+".crt"))
	if err != nil {
		return "", err
	}
	cfg.Cert = string(cert)
	key, err := ioutil.ReadFile(filepath.Join(keysPath, name+".key"))
	if err != nil {
		return "", err
	}
	cfg.Key = string(key)
	serverConfig := models.OVConfig{Profile: "default"}
	_ = serverConfig.Read("Profile")
	cfg.Port = serverConfig.Port
	cfg.Proto = serverConfig.Proto
	cfg.Auth = serverConfig.Auth
	cfg.Cipher = serverConfig.Cipher
	cfg.Keysize = serverConfig.Keysize

	destPath := filepath.Join(state.GlobalCfg.OVConfigPath, "keys", name+".ovpn")
	if err := config.SaveToFile(filepath.Join(c.ConfigDir, "openvpn-client-config.tpl"), cfg, destPath); err != nil {
		beego.Error(err)
		return "", err
	}

	return destPath, nil
}
