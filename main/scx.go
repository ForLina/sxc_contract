package main

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"strconv"
)

type Sxc struct {
}

// 附件信息
type Attachment struct {
	ID  string `json:"id"`  // 附件的唯一ID
	Md5 string `json:"md5"` // 附件的MD5
}

type Donation struct {
	Donator      string  `json:"donator"`       //捐赠者姓名 匿名/机构名称/姓名
	Amount       float64 `json:"amount"`        //捐赠金额
	SerialNumber string  `json:"serial_number"` // 业务流水号
	PlatformID   string  `json:"platform_id"`   //捐赠者在平台的ID
}

type RechargeHistory struct {
	Amount       float64 `json:"amount"`        // 充值金额
	SerialNumber string  `json:"serial_number"` // 业务流水号
}

// 筹款申请合约
type Application struct {

	// 以下9个参数申请初始化的时候需要使用
	ApplicationNumber string `json:"application_number"` // 申请编号
	Name              string `json:"name"`               //申请者姓名
	ID                string `json:"id"`                 //申请者身份证号
	HospitalCode      string `json:"hospital_code"`      // 医院的编号
	DepartmentCode    string `json:"department_code"`    //科室的编号

	StreetOfficeCode string  `json:"street_office_code"` // 街道办的编号
	CardNumber       string  `json:"card_number"`        // 就诊卡号
	DescMd5          string  `json:"desc_md5"`           // 病情描述的md5
	NeedAmount       float64 `json:"need_amount"`        // 用户申请的资金数量

	// 此项可以单独补充
	ApplicationAttachments []Attachment `json:"application_attachments"` // 用户申请的时候提交的资料

	State int //流程状态 1.待医院审核 2.医院审核不通过 3.待街道办审核 4.街道办审核不通过
	// 5.街道办审核通过 6.筹款中 7.筹款完成 8.资金发放完成

	HospitalApproveAmount float64      `json:"hospital_approve_amount"` // 医院审核同意金额
	HospitalAttachments   []Attachment `json:"hospital_attachments"`    // 医院审核的相关资料

	StreetOfficeApproveAmount float64      `json:"street_office_approve_amount"` // 街道办同意的金额
	StreetOfficeAttachments   []Attachment `json:"street_office_attachments"`    // 街道办审核的相关资料

	Donations    []Donation `json:"donations"`     // 捐赠流水
	AmountRaised float64    `json:"amount_raised"` //已经募集到的金额

	RechargeHistory []RechargeHistory `json:"recharge_history"` // 充值历史记录

	Balance float64 `json:"balance"` //合约余额
}

func (t *Sxc) Init(stub shim.ChaincodeStubInterface) peer.Response {
	return shim.Success(nil)
}

func (t *Sxc) Invoke(stub shim.ChaincodeStubInterface) peer.Response {

	fn, args := stub.GetFunctionAndParameters()

	var result string
	var err error

	if fn == "applicate" {
		result, err = applicate(stub, args)
	} else if fn == "getUserVote" {
		err = fmt.Errorf("暂时不支持此函数")
	}

	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success([]byte(result))
}



// 发起申请
// 入参列表
//		   applicationNumber 申请编号
//         name 申请者姓名
//         id 申请者身份证号
//         hospitalCode 医院编号
//         departmentCode 科室编号

//         streetOfficeCode 街道办编号
//         cardNumber 就诊卡号
//         descMd5 病情描述的md5
//         needAmount 资金需求
func applicate(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 9 {
		return "", fmt.Errorf("参数目错误，需要 9 个参数, 收到 %d 个", len(args))
	}

	application := Application{}
	applicationNumber := args[0]

	applicationAsBytes, err := stub.GetState(applicationNumber)

	if applicationAsBytes != nil {
		return "", fmt.Errorf("已经存在此合约编号 %s", args[0])
	} else {

		needAmount, err := strconv.ParseFloat(args[8],64)

		if err != nil {
			return "", fmt.Errorf("无法将需求资金转换为float64类型  %s", args[8])
		}

		//["invoke", "applicate", "1", "lyx", "500222199009214433", "995", "3", "8876", "9988123519", "abcdabcdabcdabcdabcdabcdabcdabcd", "4000.32"]
		application = Application{
			ApplicationNumber: args[0],
			Name: args[1],
			ID: args[2],
			HospitalCode: args[3],
			DepartmentCode: args[4],

			StreetOfficeCode: args[5],
			CardNumber: args[6],
			DescMd5: args[7],
			NeedAmount: needAmount,
		}
	}

	//将 Application 对象 转为 JSON 对象
	applicationJsonAsBytes, err := json.Marshal(application)
	if err != nil {
		return "", fmt.Errorf("无法将申请对象转换为Json对象")
	}

	err = stub.PutState(applicationNumber, applicationJsonAsBytes)
	if err != nil {
		return "", fmt.Errorf("申请写入账本失败")
	}

	return "成功", nil
}

func main() {
	if err := shim.Start(new(Sxc)); err != nil {
		fmt.Printf("Error starting Sxc chaincode: %s", err)
	}
}