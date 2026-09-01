package database

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

// DemoPassword is the password of every seeded account. Seeding only runs when
// the users table is empty, so this never overwrites real credentials.
const DemoPassword = "123456"

// Seed loads demo accounts and a demo product catalog. Both are idempotent:
// seeding is skipped for tables that already contain rows, which makes the
// operation safe to run on every boot.
func Seed(db *gorm.DB) error {
	if err := seedUsers(db); err != nil {
		return fmt.Errorf("seed users: %w", err)
	}
	if err := seedProducts(db); err != nil {
		return fmt.Errorf("seed products: %w", err)
	}
	return nil
}

func seedUsers(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(DemoPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	avatar := "https://img12.360buyimg.com/imagetools/jfs/t1/143702/31/16654/116794/5fc6f541Edebf8a57/4138097748889987.png"
	users := []model.User{
		{Username: "user", PasswordHash: string(hash), RealName: "测试用户", Avatar: avatar, Roles: model.Roles{model.RoleUser}, Status: "active"},
		{Username: "admin", PasswordHash: string(hash), RealName: "管理员", Avatar: avatar, Roles: model.Roles{model.RoleUser, model.RoleAdmin}, Status: "active"},
	}
	return db.Create(&users).Error
}

func seedProducts(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.Product{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	for index := range demoProducts {
		demoProducts[index].SearchText = searchText(demoProducts[index])
		if demoProducts[index].Status == "" {
			demoProducts[index].Status = model.StatusOnSale
		}
	}
	return db.CreateInBatches(demoProducts, 50).Error
}

// searchText builds the denormalised column used by keyword search. It is
// maintained on both sides (seed and write path) so that search behaves the
// same for seeded and operator-created products.
func searchText(product model.Product) string {
	return strings.Join([]string{product.Title, product.ShopName, product.ShopDesc, product.Description}, " ")
}

const (
	shopLake   = "阳澄湖大闸蟹自营店"
	shopSupor  = "苏泊尔官方自营店"
	shopHuawei = "华为官方自营店"
	shopDyson  = "戴森官方旗舰店"
	shopApple  = "Apple 产品京选店"
	shopXiaomi = "小米官方自营店"
	shopNike   = "Nike 官方旗舰店"
)

// demoProducts mirrors the catalogue shipped with the frontend mock server, so
// switching the H5 app from the mock to this service changes no visible data.
var demoProducts = []model.Product{
	{
		Title:       "【活蟹】阳澄湖大闸蟹 公4.5两 母3.5两 4对8只 鲜活生鲜螃蟹现货水产礼盒海鲜",
		ImgURL:      "https://img10.360buyimg.com/n2/s400x400_jfs/t1/210890/22/4728/163829/6163a590Eb7c6f4b5/6390526d49791cb9.jpg!q70.jpg",
		Price:       "388",
		VipPrice:    "378",
		ShopDesc:    "自营",
		Delivery:    "厂商配送",
		ShopName:    shopLake,
		Description: "新鲜捕捉，顺丰冷链配送，保证鲜活到家。公蟹膏满黄肥，母蟹籽多肉嫩。",
		Stock:       120, Sales: 1248, Featured: true, Status: model.StatusOnSale,
	},
	{
		Title:       "【礼券】阳澄湖烟雨 海鲜卡券海产提货礼品卡春节年夜饭年货生鲜过年海鲜礼盒大礼包",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s400x400_jfs/t1/181328/3/31476/203233/63b66ef1F60f5f0f8/f4e8c4b6df4194d6.jpg!q70.dpg.webp",
		Price:       "598",
		VipPrice:    "378",
		ShopDesc:    "自营",
		Delivery:    "厂商配送",
		ShopName:    shopLake,
		Description: "精选海鲜礼盒，包含多种海产品，送礼体面大方。",
		Stock:       86, Sales: 892, Featured: true, Status: model.StatusOnSale,
	},
	{
		Title:       "苏泊尔（SUPOR）电饭煲远红外加热IH本釜内胆 电饭锅4L智能预约家用煮饭锅一键柴火饭SF40HC81",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s400x400_jfs/t1/214199/39/25134/127357/63c2b3adFed9c98f4/54126e85c23d0893.jpg!q80.dpg",
		Price:       "1759",
		VipPrice:    "1749",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopSupor,
		Description: "远红外加热，IH本釜内胆，4L大容量，一键柴火饭，智能预约。",
		Stock:       52, Sales: 605, Featured: false, Status: model.StatusOnSale,
	},
	{
		Title:       "Apple/苹果 iPhone 17 256GB 白色 支持移动联通电信5G 双卡双待手机",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s1265x1265_jfs/t20280402/412326/20/12475/54533/69cf75afFc560cac6/0a02320320cce0f2.jpg!q70.dpg.webp",
		Price:       "5999",
		VipPrice:    "5999",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopApple,
		Description: "搭载A19芯片，6.1英寸超视网膜XDR显示屏，支持5G网络，双卡双待设计，提供卓越性能和流畅体验。",
		Stock:       34, Sales: 2310, Featured: true, Status: model.StatusOnSale,
	},
	{
		Title:       "戴森（Dyson）V12 Detect Slim 轻量智能无线吸尘器 激光探测",
		ImgURL:      "https://img10.360buyimg.com/pcpubliccms/s400x400_jfs/t1/414103/33/18764/70114/69eae2e4F9e33854b/00832ee3e88e35b0.jpg!q80.dpg",
		Price:       "3990",
		VipPrice:    "3890",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopDyson,
		Description: "激光探测微尘，智能调节吸力，轻巧机身仅1.5kg。",
		Stock:       27, Sales: 392, Featured: false, Status: model.StatusOnSale,
	},
	{
		Title:       "小米（MI）Redmi Note 15 Pro 5G 智能手机 12GB+256GB 星空蓝",
		ImgURL:      "https://img10.360buyimg.com/n2/s400x400_jfs/t1/210890/22/4728/163829/6163a590Eb7c6f4b5/6390526d49791cb9.jpg!q70.jpg",
		Price:       "1599",
		VipPrice:    "1499",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopXiaomi,
		Description: "天玑7400-Ultra 处理器，1.5K 高光护眼屏，5110mAh 大电池，IP68 防尘防水。",
		Stock:       210, Sales: 1865, Featured: true, Status: model.StatusOnSale,
	},
	{
		Title:       "华为（HUAWEI）MatePad 11.5 英寸平板电脑 8GB+256GB WIFI 曜石黑",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s400x400_jfs/t1/214199/39/25134/127357/63c2b3adFed9c98f4/54126e85c23d0893.jpg!q80.dpg",
		Price:       "2199",
		VipPrice:    "2099",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopHuawei,
		Description: "2.2K 高清护眼屏，120Hz 刷新率，四扬声器环绕音效，支持多屏协同。",
		Stock:       96, Sales: 748, Featured: false, Status: model.StatusOnSale,
	},
	{
		Title:       "Nike 耐克 AIR ZOOM 男子跑步鞋 透气缓震运动鞋 黑白配色",
		ImgURL:      "https://img10.360buyimg.com/pcpubliccms/s400x400_jfs/t1/414103/33/18764/70114/69eae2e4F9e33854b/00832ee3e88e35b0.jpg!q80.dpg",
		Price:       "899",
		VipPrice:    "799",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopNike,
		Description: "工程网眼鞋面提升透气性，Zoom Air 气垫提供灵敏缓震，橡胶外底耐磨抓地。",
		Stock:       144, Sales: 1108, Featured: false, Status: model.StatusOnSale,
	},
	{
		Title:       "阳澄湖大闸蟹 现货礼盒装 公4两 母3两 3对6只 生鲜水产",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s400x400_jfs/t1/181328/3/31476/203233/63b66ef1F60f5f0f8/f4e8c4b6df4194d6.jpg!q70.dpg.webp",
		Price:       "268",
		VipPrice:    "258",
		ShopDesc:    "自营",
		Delivery:    "厂商配送",
		ShopName:    shopLake,
		Description: "产地直发，泡沫箱加冰袋保鲜，48 小时内送达，死蟹包赔。",
		Stock:       0, Sales: 516, Featured: false, Status: model.StatusSoldOut,
	},
	{
		Title:       "苏泊尔（SUPOR）不粘锅麦饭石色炒锅 32cm 电磁炉燃气灶通用",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s400x400_jfs/t1/214199/39/25134/127357/63c2b3adFed9c98f4/54126e85c23d0893.jpg!q80.dpg",
		Price:       "199",
		VipPrice:    "179",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopSupor,
		Description: "麦饭石色不粘涂层，少油易清洗，加厚复合锅底导热均匀，适配多种灶具。",
		Stock:       320, Sales: 975, Featured: false, Status: model.StatusOnSale,
	},
	{
		Title:       "Apple iPad Air 11 英寸 M3 芯片 128GB WLAN 版 深空灰色",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s1265x1265_jfs/t20280402/412326/20/12475/54533/69cf75afFc560cac6/0a02320320cce0f2.jpg!q70.dpg.webp",
		Price:       "4799",
		VipPrice:    "4699",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopApple,
		Description: "M3 芯片带来强劲性能，Liquid 视网膜显示屏，支持 Apple Pencil Pro 与妙控键盘。",
		Stock:       63, Sales: 667, Featured: true, Status: model.StatusOnSale,
	},
	{
		Title:       "戴森（Dyson）Supersonic 电吹风 HD08 智能温控 恒温护发",
		ImgURL:      "https://img10.360buyimg.com/pcpubliccms/s400x400_jfs/t1/414103/33/18764/70114/69eae2e4F9e33854b/00832ee3e88e35b0.jpg!q80.dpg",
		Price:       "2999",
		VipPrice:    "2899",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopDyson,
		Description: "智能温控每秒测量风温 20 余次，防止过热损伤发质，搭配多种造型风嘴。",
		Stock:       48, Sales: 431, Featured: false, Status: model.StatusOnSale,
	},
	{
		Title:       "小米（MI）米家空气净化器 4 Pro 家用除甲醛除菌 智能除雾霾",
		ImgURL:      "https://img10.360buyimg.com/n2/s400x400_jfs/t1/210890/22/4728/163829/6163a590Eb7c6f4b5/6390526d49791cb9.jpg!q70.jpg",
		Price:       "1499",
		VipPrice:    "1399",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopXiaomi,
		Description: "OLED 显示屏实时反馈空气质量，除甲醛、抗病毒、抗过敏原三重滤芯设计。",
		Stock:       75, Sales: 1034, Featured: false, Status: model.StatusOnSale,
	},
	{
		Title:       "Nike 耐克 男子运动短袖速干 T 恤 训练健身上衣 透气排汗",
		ImgURL:      "https://m.360buyimg.com/mobilecms/s400x400_jfs/t1/181328/3/31476/203233/63b66ef1F60f5f0f8/f4e8c4b6df4194d6.jpg!q70.dpg.webp",
		Price:       "249",
		VipPrice:    "229",
		ShopDesc:    "自营",
		Delivery:    "京东物流",
		ShopName:    shopNike,
		Description: "Dri-FIT 速干面料帮助保持干爽，平缝设计减少摩擦，适合高强度训练。",
		Stock:       132, Sales: 1560, Featured: false, Status: model.StatusOnSale,
	},
}
