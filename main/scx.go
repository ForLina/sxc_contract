package main

// 业务说明
// 1.填写申请
// 2.医院审核
// 3.开始筹款
// 4.签订贷款协议
// 5.放款
// 6.欺诈判定 提交欺诈材料 停止为用户偿还贷款
// 7.为用户偿还贷款

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"strconv"
)

// 业务流程状态
const (
	HospitalVerify     = 1 // 等待医院审核
	HospitalReject     = 2 // 医院审核不通过
	Raising            = 3 // 筹款中
	Raised             = 4 // 筹款完成 筹集到了指定金额
	Cheat              = 5 // 涉及合约欺诈
	RepaymentCompleted = 6 // 还款完成
)

const (
	Agree  = "1"
	Reject = "0"
)

type Sxc struct {
}

// 附件信息
type Attachment struct {
	ID  string `json:"id"`  // 附件的唯一ID
	Md5 string `json:"md5"` // 附件的MD5
}

// 捐赠信息
type Donation struct {
	Donator      string  `json:"donator"`       //捐赠者姓名 匿名/机构名称/姓名
	Amount       float64 `json:"amount"`        //捐赠金额
	SerialNumber string  `json:"serial_number"` // 业务流水号 此流水号可以在用户对应的充值系统中查询 例如 支付宝 微信中查询
	PlatformID   string  `json:"platform_id"`   //捐赠者在平台的ID
}

// 贷款信息
// 还款方式默认等额本息
type LoanInfo struct {
	LoanNumber       string `json:"loan_number"`       // 贷款单号
	FirstRepayment   string `json:"first_repayment"`   // 第一次还款的月份
	TotalMonth       string `json:"total_month"`       // 总共需要还款多少期
	MoneyReceived    bool   `json:"money_received"`    // 是否已经收到放款
	ReceiveSerialNumber string `json:"receive_serial_number"` // 收款流水号
	RepaymentHistory string `json:"repayment_history"` // 还款历史列表 存储还款流水号即可
	LoanAmount float64 `json:"loan_amount"` // 贷款金额
}

// 充值信息
type RechargeHistory struct {
	Amount       float64 `json:"amount"`        // 充值金额
	SerialNumber string  `json:"serial_number"` // 业务流水号 此流水号可以对应在医院的系统中查询到
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

	State int // 参考常量定义 业务流程状态

	HospitalApproveAmount float64      `json:"hospital_approve_amount"` // 医院审核同意金额
	HospitalOperator      string       `json:"hospital_operator"`       // 医院的审核员
	HospitalAttachments   []Attachment `json:"hospital_attachments"`    // 医院审核的相关资料

	DonateCounter int     `json:"donate_counter"` // 捐赠计数器
	AmountRaised  float64 `json:"amount_raised"`  //已经募集到的金额

	// 贷款信息
	LoanCounter int     `json:"loan_counter"` //贷款计数器 可以多次贷款
	LoanTotal   float64 `json:"loan_total"`   //总共已经贷款多少
	ReceivedLoanTotal float64  `json:"received_loan_total"` // 总共已经收到银行放款的总额度

	// 充值到就诊卡的信息
	RechargeCounter int `json:"recharge_counter"` // 充值计数器
	RechargeTotal float64 `json:"recharge_total"` // 累计充值金额

	Balance float64 `json:"balance"` //合约余额
}

func (t *Sxc) Init(	stub shim.ChaincodeStubInterface) peer.Response {
	return shim.Success(nil)
}

func (t *Sxc) Invoke(stub shim.ChaincodeStubInterface) peer.Response {

	fn, args := stub.GetFunctionAndParameters()

	var result string
	var err error

	switch fn {
	case "applicate":
		result, err = applicate(stub, args)
	case "hVerify":
		result, err = hVerify(stub, args)
	case "donate":
		result, err = donate(stub, args)
	case "getRaised":
		result, err = getRaised(stub, args)
	case "loan":
		result, err = loan(stub, args)
	case "receivedLoan":
		result, err = receivedLoan(stub, args)
	case "setCheat":
		result, err = setCheat(stub, args)
	case "recharge":
		result, err = recharge(stub, args)
	case "getApplicationInfo":
		result, err = getApplicationInfo(stub, args)
	default:
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

//请求示例 ["invoke", "applicate", "1", "lyx", "500222199009214433", "995", "3", "8876", "9988123519", "abcdabcdabcdabcdabcdabcdabcdabcd", "4000.32"]

func applicate(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 9 {
		return "", fmt.Errorf("参数目错误，需要 9 个参数, 收到 %d 个", len(args))
	}

	application := Application{}
	applicationNumber := args[0]

	applicationAsBytes, err := stub.GetState(applicationNumber)

	if err != nil {
		return "", fmt.Errorf("获取账本状态失败 %s", applicationNumber)
	}

	if applicationAsBytes != nil {
		return "", fmt.Errorf("已经存在此合约编号 %s", args[0])
	} else {

		needAmount, err := strconv.ParseFloat(args[8], 64)

		if err != nil {
			return "", fmt.Errorf("无法将需求资金转换为float64类型  %s", args[8])
		}

		application = Application{
			ApplicationNumber: args[0],
			Name:              args[1],
			ID:                args[2],
			HospitalCode:      args[3],
			DepartmentCode:    args[4],

			StreetOfficeCode: args[5],
			CardNumber:       args[6],
			DescMd5:          args[7],
			NeedAmount:       needAmount,

			State: HospitalVerify,
		}
	}

	_, err = write(stub, application)
	if err != nil {
		return "", err
	}

	return "成功", nil
}

// 医院审核
// 入参列表
//          application_number 合约编号
//			operator 审核人员姓名
// 		    agree 是否同意 0不同意 1同意
//          approveAmount 同意的金额
//          attachments 附件列表 json string [{"id":string}, {"md5":string}]

// 范例 ["invoke", "hVerify", "1", "lengtingxue", "1", "3500", "[{\"id\":\"attachment_id1\", \"md5\":\"md123456md123456md123456md123456\"}]"]
func hVerify(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 5 {
		return "", fmt.Errorf("参数目错误，需要 5 个参数, 收到 %d 个", len(args))
	}

	applicationNumber := args[0]
	application, err := getApplication(stub, applicationNumber)

	if err != nil {
		return "", err
	}

	if application.State != HospitalVerify {
		return "", fmt.Errorf("合约已经审核过啦")
	}

	approveAmount, err := strconv.ParseFloat(args[3], 64)

	if err != nil {
		return "", fmt.Errorf("无法将同意金额转换为float64类型  %s", args[3])
	}

	var attachments []Attachment
	err = json.Unmarshal([]byte(args[4]), &attachments)
	if err != nil {
		return "", fmt.Errorf("无法将附件列表转换为附件对象 %s", args[4])
	}

	if args[2] == Reject {
		application.State = HospitalReject //审核不通过
	} else if args[2] == Agree {
		application.State = Raising // 开始筹款
		application.HospitalApproveAmount = approveAmount
	} else {
		return "", fmt.Errorf("同意与否参数错误 %s", args[2])
	}

	application.HospitalAttachments = attachments
	application.HospitalOperator = args[1]

	_, err = write(stub, application)
	if err != nil {
		return "", err
	}

	return "成功", nil
}

// 捐赠
// 入参列表
//          application_number 合约编号
//			donator 赠者姓名 匿名/机构名称/姓名
// 		    amount 捐赠金额
//          serialNumber 业务流水号
//          platformID 捐赠者的平台ID

// 范例 ["invoke", "donate", "1", "zhangsan", "300", "sxc202008161449", "platformid008"]
func donate(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 5 {
		return "", fmt.Errorf("参数目错误，需要 5 个参数, 收到 %d 个", len(args))
	}

	applicationNumber := args[0]
	application, err := getApplication(stub, applicationNumber)
	if err != nil {
		return "", err
	}

	if application.State != Raising {
		return "", fmt.Errorf("当前合约不能接受捐赠")
	}

	// 捐赠金额
	donateAmount, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return "", fmt.Errorf("无法将同意金额转换为float64类型  %s", args[2])
	}

	if donateAmount <= 0 {
		return "", fmt.Errorf("捐赠金额必须大于等于0")
	}

	donateHistory := Donation{
		Donator:      args[1],
		Amount:       donateAmount,
		SerialNumber: args[3],
		PlatformID:   args[4]}

	donateCounter := application.DonateCounter + 1
	strDonateCounter := strconv.Itoa(donateCounter)

	donateKey := applicationNumber + "," + strDonateCounter

	donationJsonAsBytes, err := json.Marshal(donateHistory)
	if err != nil {
		return "", fmt.Errorf("无法将捐赠历史对象转换为Json对象")
	}

	err = stub.PutState(donateKey, donationJsonAsBytes)
	if err != nil {
		return "", fmt.Errorf("捐赠历史写入账本失败")
	}

	// 更新捐赠次数计数器
	application.DonateCounter = donateCounter

	// 更新金额
	application.AmountRaised = application.AmountRaised + donateAmount

	// 更新余额
	application.Balance = application.Balance + donateAmount

	_, err = write(stub, application)
	if err != nil {
		return "", err
	}

	return "成功", nil
}

// 查询申请合约的总捐赠额度
// 入参
//			application_number 合约编号
// 范例 ["invoke", "getRaised", "1"]
func getRaised(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("参数目错误，需要 1 个参数, 收到 %d 个", len(args))
	}

	application := Application{}
	applicationNumber := args[0]

	applicationAsBytes, err := stub.GetState(applicationNumber)

	if err != nil {
		return "", fmt.Errorf("获取账本状态失败 %s", applicationNumber)
	}

	if applicationAsBytes == nil {
		return "", fmt.Errorf("未找到此申请的信息 %s", args[0])
	}

	err = json.Unmarshal(applicationAsBytes, &application)
	if err != nil {
		return "", fmt.Errorf("将合约转换为json对象失败")
	}

	strAmount := strconv.FormatFloat(application.AmountRaised, 'E', -1, 64)
	return strAmount, nil

}

// 贷款
// 入参列表
//          application_number 合约编号
// 		    amount 贷款金额
//          loan_number 贷款单号
//          first_repayment 第一次还款的月份
//  		total_month 总共需要还款多少期

// 范例 ["invoke", "loan", "1", "200", "sxc202008161449", "2020-09", "24"]
func loan(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 5 {
		return "", fmt.Errorf("参数目错误，需要 5 个参数, 收到 %d 个", len(args))
	}

	applicationNumber := args[0]
	application, err := getApplication(stub, applicationNumber)
	if err != nil {
		return "", err
	}

	if application.State != Raising && application.State != Raised {
		return "", fmt.Errorf("当前合约不能申请贷款")
	}

	// 贷款金额
	loanAmount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return "", fmt.Errorf("无法将贷款金额转换为float64类型  %s", args[1])
	}
	if loanAmount <= 0 {
		return "", fmt.Errorf("贷款金额需要是正数  %s", args[1])
	}
	if loanAmount+application.LoanTotal > application.AmountRaised {
		return "", fmt.Errorf("贷款金额不能超过已经募集到了的金额  %s", args[1])
	}

	loanInfo := LoanInfo{
		LoanNumber:       args[2],
		FirstRepayment:   args[3],
		TotalMonth:       args[4],
		MoneyReceived:    false,
		RepaymentHistory: "[]",
		LoanAmount: loanAmount}

	loanCounter := application.LoanCounter + 1
	strLoanCounter := strconv.Itoa(loanCounter)


	err = setLoanInfo(stub, applicationNumber, strLoanCounter, loanInfo)
	if err != nil {
		return "贷款信息写入合约失败", err
	}

	// 更新捐赠次数计数器
	application.LoanCounter = loanCounter
	// 更新总贷款金额
	application.LoanTotal = application.LoanTotal + loanAmount

	_, err = write(stub, application)
	if err != nil {
		return "", err
	}

	returnStr := "{\"counter\":" + strconv.Itoa(loanCounter) + "}"
	return returnStr, nil
}

// 收到银行放款
// 入参列表
//          application_number 合约编号
// 		    loan_number 贷款单号
//          load_counter 计数器
//          serial_number 放款入账流水号

// 范例 ["invoke", "receivedLoan", "1", "sxc202008161449", "1", "serial_number2020-08-22 20:31:06"]
func receivedLoan(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 4 {
		return "", fmt.Errorf("参数目错误，需要 4 个参数, 收到 %d 个", len(args))
	}

	applicationNumber := args[0]
	application, err := getApplication(stub, applicationNumber)
	if err != nil {
		return "", err
	}

	if application.State == Cheat {
		return "", fmt.Errorf("涉嫌欺诈,不予放款")
	}

	strLoanCounter := args[2]
	loanInfo, err := getLoanInfo(stub, applicationNumber, strLoanCounter)
	if err != nil {
		return "获取贷款信息失败", err
	}

	if loanInfo.LoanNumber != args[1] {
		return "贷款单号不匹配", fmt.Errorf("贷款单号不匹配")
	}

	if loanInfo.MoneyReceived {
		return "已经收到放款", fmt.Errorf("已经收到放款")
	}

	loanInfo.MoneyReceived = true
	loanInfo.ReceiveSerialNumber = args[3]

	err = setLoanInfo(stub, applicationNumber, strLoanCounter, loanInfo)
	if err != nil {
		return "贷款信息写入合约失败", err
	}

	application.ReceivedLoanTotal = application.ReceivedLoanTotal + loanInfo.LoanAmount
	_, err = write(stub, application)
	if err != nil {
		return "", err
	}

	return "成功", nil
}

// 设置此申请为欺诈申请
// 入参列表
//          application_number 合约编号
func setCheat(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("参数目错误，需要 1 个参数, 收到 %d 个", len(args))
	}

	applicationNumber := args[0]
	application, err := getApplication(stub, applicationNumber)
	if err != nil {
		return "", err
	}

	application.State = Cheat

	_, err = write(stub, application)
	if err != nil {
		return "", err
	}

	return "成功", nil
}

// 为用户的就诊卡充值
// 入参列表
//          application_number 合约编号
//          serial_number 充值流水号
//          amount 充值金额

// 范例 ["invoke", "recharge", "1", "sxc202008161449", "100"]
func recharge(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("参数目错误，需要 3 个参数, 收到 %d 个", len(args))
	}

	applicationNumber := args[0]
	application, err := getApplication(stub, applicationNumber)
	if err != nil {
		return "", err
	}

	if application.State == Cheat {
		return "", fmt.Errorf("涉嫌欺诈,不予充值")
	}

	amount, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return "", fmt.Errorf("无法将充值金额转换为float64类型  %s", args[1])
	}

	newCounter := application.RechargeCounter + 1

	rechargeHistory := RechargeHistory{
		Amount:amount,
		SerialNumber:args[2],
	}

	rechargeKey := applicationNumber + "," + strconv.Itoa(newCounter)

	rechargeHistoryJsonAsBytes, err := json.Marshal(rechargeHistory)
	if err != nil {
		return "", fmt.Errorf("无法将申请对象转换为Json对象")
	}

	err = stub.PutState(rechargeKey, rechargeHistoryJsonAsBytes)
	if err != nil {
		return "", fmt.Errorf("充值历史写入账本失败")
	}

	application.RechargeTotal = application.RechargeTotal + amount
	application.RechargeCounter = newCounter
	_, err = write(stub, application)
	if err != nil {
		return "", err
	}

	return "成功", nil
}

// 获取合约详情
// 入参列表
//          application_number 合约编号

// 范例 ["invoke", "getApplicationInfo", "1"]
func getApplicationInfo(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("参数目错误，需要 1 个参数, 收到 %d 个", len(args))
	}

	applicationNumber := args[0]
	applicationAsBytes, err := stub.GetState(applicationNumber)

	if err != nil {
		return "获取账本状态失败", fmt.Errorf("获取账本状态失败 %s", applicationNumber)
	}

	if applicationAsBytes == nil {
		return "未找到此申请的信息", fmt.Errorf("未找到此申请的信息 %s", applicationNumber)
	}

	return string(applicationAsBytes), nil
}

// 将Application 对象作为字符串写入合约
func write(stub shim.ChaincodeStubInterface, application Application) (string, error) {
	//将 Application 对象 转为 JSON 对象
	applicationJsonAsBytes, err := json.Marshal(application)
	if err != nil {
		return "", fmt.Errorf("无法将申请对象转换为Json对象")
	}

	err = stub.PutState(application.ApplicationNumber, applicationJsonAsBytes)
	if err != nil {
		return "", fmt.Errorf("申请写入账本失败")
	}

	return "", nil
}

func getApplication(stub shim.ChaincodeStubInterface, applicationNumber string) (Application, error) {
	application := Application{}

	applicationAsBytes, err := stub.GetState(applicationNumber)

	if err != nil {
		return application, fmt.Errorf("获取账本状态失败 %s", applicationNumber)
	}

	if applicationAsBytes == nil {
		return application, fmt.Errorf("未找到此申请的信息 %s", applicationNumber)
	}

	err = json.Unmarshal(applicationAsBytes, &application)
	if err != nil {
		return application, fmt.Errorf("将合约转换为json对象失败")
	}

	return application, nil
}

func getLoanInfo(stub shim.ChaincodeStubInterface, applicationNumber string, loanCounter string) (LoanInfo, error){
	loanInfo := LoanInfo{}
	loanKey := applicationNumber + "," + loanCounter
	loanInfoAsBytes, err := stub.GetState(loanKey)
	if err != nil {
		return loanInfo, fmt.Errorf("获取贷款信息失败 %s,%s", applicationNumber, loanCounter)
	}
	err = json.Unmarshal(loanInfoAsBytes, &loanInfo)
	if err != nil {
		return loanInfo, fmt.Errorf("贷款信息json串转换为贷款信息对象失败")
	}
	return loanInfo, nil
}

func setLoanInfo(stub shim.ChaincodeStubInterface, applicationNumber string, loanCounter string, loanInfo LoanInfo) error {
	loanKey := applicationNumber + "," + loanCounter

	loanJsonAsBytes, err := json.Marshal(loanInfo)
	if err != nil {
		return fmt.Errorf("无法贷款信息转换为Json字符串")
	}

	err = stub.PutState(loanKey, loanJsonAsBytes)
	if err != nil {
		return fmt.Errorf("贷款信心写入账本失败")
	}
	return nil
}

func main() {
	if err := shim.Start(new(Sxc)); err != nil {
		fmt.Printf("Error starting Sxc chaincode: %s", err)
	}
}
