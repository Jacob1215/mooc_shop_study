package initialize

import (
	"encoding/json"
	"fmt"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"mxshop_srvs/order_srv/global"
)

//配置信息初始化
func GetEnvInfo(env string) bool { //测试环境和生产环境隔离开
	viper.AutomaticEnv()
	return viper.GetBool(env)
	//刚才设置的环境变量想要生效 ，我们必须得重启goland。
}
func InitConfig() {
	//从配置文件中读取对应的配置//这个配置只读取数据库
	debug := GetEnvInfo("mxshop_debug")
	configFilePrefix := "config"
	configFileName := fmt.Sprintf("order_srv/%s-pro.yaml", configFilePrefix)
	if debug {
		configFileName = fmt.Sprintf("order_srv/%s-debug.yaml", configFilePrefix)
	}
	v := viper.New() //取数据比较简单
	//文件的路径如何设置
	v.SetConfigFile(configFileName) //路径设置，一个大的问题。//这样就写成了绝对路径，还有很多方法，但是比较麻烦。
	//还可以改path，但比较麻烦，并没用。
	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}
	//这个对象如何在其他文件中使用 - 全局变量
	//serverConfig :=config.ServerConfig{}//配置了global就不用写这条了
	if err := v.Unmarshal(&global.NacosConfig); err != nil {
		panic(err)
	}
	//fmt.Println(global.ServerConfig)
	zap.S().Infof("配置信息: %v", global.NacosConfig)

	//从nacos中读取配置信息
	sc := []constant.ServerConfig{
		{
			IpAddr: global.NacosConfig.Host,
			Port:   global.NacosConfig.Port,
		},
	}
	cc := constant.ClientConfig{
		NamespaceId:         global.NacosConfig.Namespace, //nacos拿的。 //we can create multiple clients with different namespaceId to support multiple namespace.When namespace is public, fill in the blank string here.
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "tmp/nacos/log",
		CacheDir:            "tmp/nacos/cache",
		RotateTime:          "1h",
		MaxAge:              3,
		LogLevel:            "debug",
	} //其他默认
	// Another way of create config client for dynamic configuration (recommend)
	configClient, err := clients.NewConfigClient( //这里跟老师不一样
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		panic(err)
	}
	//获取配置
	content, err := configClient.GetConfig(vo.ConfigParam{
		DataId: global.NacosConfig.DataId,
		Group:  global.NacosConfig.Group})
	if err != nil {
		panic(err)
	}
	//json转struct。
	//想要将一个字符串转换成struct，需要去设置这个struct的tag
	err = json.Unmarshal([]byte(content), &global.ServerConfig)
	if err != nil {
		zap.S().Fatalf("读取nacos配置失败: %s", err)
	}
	fmt.Println(&global.ServerConfig)

}
