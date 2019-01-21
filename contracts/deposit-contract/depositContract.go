// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package depositcontract

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// DepositContractABI is the input ABI used to generate the binding from.
const DepositContractABI = "[{\"name\":\"Deposit\",\"inputs\":[{\"type\":\"bytes32\",\"name\":\"previous_deposit_root\",\"indexed\":false},{\"type\":\"bytes\",\"name\":\"data\",\"indexed\":false},{\"type\":\"bytes\",\"name\":\"merkle_tree_index\",\"indexed\":false}],\"anonymous\":false,\"type\":\"event\"},{\"name\":\"ChainStart\",\"inputs\":[{\"type\":\"bytes32\",\"name\":\"deposit_root\",\"indexed\":false},{\"type\":\"bytes\",\"name\":\"time\",\"indexed\":false}],\"anonymous\":false,\"type\":\"event\"},{\"name\":\"__init__\",\"outputs\":[],\"inputs\":[{\"type\":\"uint256\",\"name\":\"depositThreshold\"}],\"constant\":false,\"payable\":false,\"type\":\"constructor\"},{\"name\":\"get_deposit_root\",\"outputs\":[{\"type\":\"bytes32\",\"name\":\"out\"}],\"inputs\":[],\"constant\":true,\"payable\":false,\"type\":\"function\",\"gas\":625},{\"name\":\"deposit\",\"outputs\":[],\"inputs\":[{\"type\":\"bytes\",\"name\":\"deposit_input\"}],\"constant\":false,\"payable\":true,\"type\":\"function\",\"gas\":1707235},{\"name\":\"get_branch\",\"outputs\":[{\"type\":\"bytes32[32]\",\"name\":\"out\"}],\"inputs\":[{\"type\":\"uint256\",\"name\":\"leaf\"}],\"constant\":true,\"payable\":false,\"type\":\"function\",\"gas\":20138}]"

// DepositContractBin is the compiled bytecode used for deploying new contracts.
const DepositContractBin = `0x600035601c52740100000000000000000000000000000000000000006020526f7fffffffffffffffffffffffffffffff6040527fffffffffffffffffffffffffffffffff8000000000000000000000000000000060605274012a05f1fffffffffffffffffffffffffdabf41c006080527ffffffffffffffffffffffffed5fa0e000000000000000000000000000000000060a05260206123176101403934156100a757600080fd5b610140516000556122ff56600035601c52740100000000000000000000000000000000000000006020526f7fffffffffffffffffffffffffffffff6040527fffffffffffffffffffffffffffffffff8000000000000000000000000000000060605274012a05f1fffffffffffffffffffffffffdabf41c006080527ffffffffffffffffffffffffed5fa0e000000000000000000000000000000000060a0526000156101d5575b61016052610140526018600860208206610220016000610140516020826101c0010152602081019050806101c0526101c090505182840111156100dd57600080fd5b602080610240826020602088068803016000610140516020826101c0010152602081019050806101c0526101c0905001600060046015f15050818152809050905090508051602001806102e0828460006004600a8704601201f161014057600080fd5b50506103406102e0516008818352015b60086103405111156101615761017d565b60006103405161030001535b8151600101808352811415610150575b505060206102c05260406102e0510160206001820306601f82010390506102a0525b60006102a0511115156101b1576101cd565b60206102a051036102c0015160206102a051036102a05261019f565b610160515650005b63c5f2892f60005114156102085734156101ee57600080fd5b6001600160e05260c052604060c0205460005260206000f3005b6398b1e06a60005114156121905760206004610140376108206004356004016101603761080060043560040135111561024057600080fd5b633b9aca006109c0526109c05161025657600080fd5b6109c05134046109a052633b9aca006109a051101561027457600080fd5b6407735940006109a051111561028957600080fd5b426109e052600061016051610180516101a0516101c0516101e05161020051610220516102405161026051610280516102a0516102c0516102e05161030051610320516103405161036051610380516103a0516103c0516103e05161040051610420516104405161046051610480516104a0516104c0516104e05161050051610520516105405161056051610580516105a0516105c0516105e05161060051610620516106405161066051610680516106a0516106c0516106e05161070051610720516107405161076051610780516107a0516107c0516107e05161080051610820516108405161086051610880516108a0516108c0516108e05161090051610920516109405161096051610980516109a0516109c0516109e051610a0051610a2051610a4051610a6051610a8051610aa051610ac051610ae051610b0051610b2051610b4051610b6051610b8051610ba051610bc051610be051610c0051610c2051610c4051610c6051610c8051610ca051610cc051610ce051610d0051610d2051610d4051610d6051610d8051610da051610dc051610de051610e0051610e2051610e4051610e6051610e8051610ea051610ec051610ee051610f0051610f2051610f4051610f6051610f8051610fa051610fc051610fe05161100051611020516110405161106051611080516110a0516110c0516110e05161110051611120516111405161116051611180516111a0516111c0516111e05161120051611220516112405163ebe00197611260526109a05161128052611280516006580161009b565b506112e0526000611340525b6112e05160206001820306601f8201039050611340511015156104fc57610515565b61134051611300015261134051602001611340526104da565b6112405261122052611200526111e0526111c0526111a05261118052611160526111405261112052611100526110e0526110c0526110a0526110805261106052611040526110205261100052610fe052610fc052610fa052610f8052610f6052610f4052610f2052610f0052610ee052610ec052610ea052610e8052610e6052610e4052610e2052610e0052610de052610dc052610da052610d8052610d6052610d4052610d2052610d0052610ce052610cc052610ca052610c8052610c6052610c4052610c2052610c0052610be052610bc052610ba052610b8052610b6052610b4052610b2052610b0052610ae052610ac052610aa052610a8052610a6052610a4052610a2052610a00526109e0526109c0526109a05261098052610960526109405261092052610900526108e0526108c0526108a05261088052610860526108405261082052610800526107e0526107c0526107a05261078052610760526107405261072052610700526106e0526106c0526106a05261068052610660526106405261062052610600526105e0526105c0526105a05261058052610560526105405261052052610500526104e0526104c0526104a05261048052610460526104405261042052610400526103e0526103c0526103a05261038052610360526103405261032052610300526102e0526102c0526102a05261028052610260526102405261022052610200526101e0526101c0526101a05261018052610160526112e060088060208461146001018260208501600060046012f150508051820191505061016051610180516101a0516101c0516101e05161020051610220516102405161026051610280516102a0516102c0516102e05161030051610320516103405161036051610380516103a0516103c0516103e05161040051610420516104405161046051610480516104a0516104c0516104e05161050051610520516105405161056051610580516105a0516105c0516105e05161060051610620516106405161066051610680516106a0516106c0516106e05161070051610720516107405161076051610780516107a0516107c0516107e05161080051610820516108405161086051610880516108a0516108c0516108e05161090051610920516109405161096051610980516109a0516109c0516109e051610a0051610a2051610a4051610a6051610a8051610aa051610ac051610ae051610b0051610b2051610b4051610b6051610b8051610ba051610bc051610be051610c0051610c2051610c4051610c6051610c8051610ca051610cc051610ce051610d0051610d2051610d4051610d6051610d8051610da051610dc051610de051610e0051610e2051610e4051610e6051610e8051610ea051610ec051610ee051610f0051610f2051610f4051610f6051610f8051610fa051610fc051610fe05161100051611020516110405161106051611080516110a0516110c0516110e05161110051611120516111405161116051611180516111a0516111c0516111e05161120051611220516112405161126051611280516112a0516112c0516112e05161130051611320516113405163ebe00197611360526109e05161138052611380516006580161009b565b506113e0526000611440525b6113e05160206001820306601f8201039050611440511015156109e4576109fd565b61144051611400015261144051602001611440526109c2565b6113405261132052611300526112e0526112c0526112a05261128052611260526112405261122052611200526111e0526111c0526111a05261118052611160526111405261112052611100526110e0526110c0526110a0526110805261106052611040526110205261100052610fe052610fc052610fa052610f8052610f6052610f4052610f2052610f0052610ee052610ec052610ea052610e8052610e6052610e4052610e2052610e0052610de052610dc052610da052610d8052610d6052610d4052610d2052610d0052610ce052610cc052610ca052610c8052610c6052610c4052610c2052610c0052610be052610bc052610ba052610b8052610b6052610b4052610b2052610b0052610ae052610ac052610aa052610a8052610a6052610a4052610a2052610a00526109e0526109c0526109a05261098052610960526109405261092052610900526108e0526108c0526108a05261088052610860526108405261082052610800526107e0526107c0526107a05261078052610760526107405261072052610700526106e0526106c0526106a05261068052610660526106405261062052610600526105e0526105c0526105a05261058052610560526105405261052052610500526104e0526104c0526104a05261048052610460526104405261042052610400526103e0526103c0526103a05261038052610360526103405261032052610300526102e0526102c0526102a05261028052610260526102405261022052610200526101e0526101c0526101a05261018052610160526113e060088060208461146001018260208501600060046012f150508051820191505061016061080080602084611460010182602085016000600460def150508051820191505080611460526114609050805160200180610a00828460006004600a8704601201f1610cae57600080fd5b5050600254640100000000600254011015610cc857600080fd5b64010000000060025401611cc05261016051610180516101a0516101c0516101e05161020051610220516102405161026051610280516102a0516102c0516102e05161030051610320516103405161036051610380516103a0516103c0516103e05161040051610420516104405161046051610480516104a0516104c0516104e05161050051610520516105405161056051610580516105a0516105c0516105e05161060051610620516106405161066051610680516106a0516106c0516106e05161070051610720516107405161076051610780516107a0516107c0516107e05161080051610820516108405161086051610880516108a0516108c0516108e05161090051610920516109405161096051610980516109a0516109c0516109e051610a0051610a2051610a4051610a6051610a8051610aa051610ac051610ae051610b0051610b2051610b4051610b6051610b8051610ba051610bc051610be051610c0051610c2051610c4051610c6051610c8051610ca051610cc051610ce051610d0051610d2051610d4051610d6051610d8051610da051610dc051610de051610e0051610e2051610e4051610e6051610e8051610ea051610ec051610ee051610f0051610f2051610f4051610f6051610f8051610fa051610fc051610fe05161100051611020516110405161106051611080516110a0516110c0516110e05161110051611120516111405161116051611180516111a0516111c0516111e05161120051611220516112405161126051611280516112a0516112c0516112e05161130051611320516113405161136051611380516113a0516113c0516113e05161140051611420516114405161146051611480516114a0516114c0516114e05161150051611520516115405161156051611580516115a0516115c0516115e05161160051611620516116405161166051611680516116a0516116c0516116e05161170051611720516117405161176051611780516117a0516117c0516117e05161180051611820516118405161186051611880516118a0516118c0516118e05161190051611920516119405161196051611980516119a0516119c0516119e051611a0051611a2051611a4051611a6051611a8051611aa051611ac051611ae051611b0051611b2051611b4051611b6051611b8051611ba051611bc051611be051611c0051611c2051611c4051611c6051611c8051611ca051611cc051611ce051611d0051611d205163ebe00197611d4052611cc051611d6052611d60516006580161009b565b50611dc0526000611e20525b611dc05160206001820306601f8201039050611e205110151561109e576110b7565b611e2051611de00152611e2051602001611e205261107c565b611d2052611d0052611ce052611cc052611ca052611c8052611c6052611c4052611c2052611c0052611be052611bc052611ba052611b8052611b6052611b4052611b2052611b0052611ae052611ac052611aa052611a8052611a6052611a4052611a2052611a00526119e0526119c0526119a05261198052611960526119405261192052611900526118e0526118c0526118a05261188052611860526118405261182052611800526117e0526117c0526117a05261178052611760526117405261172052611700526116e0526116c0526116a05261168052611660526116405261162052611600526115e0526115c0526115a05261158052611560526115405261152052611500526114e0526114c0526114a05261148052611460526114405261142052611400526113e0526113c0526113a05261138052611360526113405261132052611300526112e0526112c0526112a05261128052611260526112405261122052611200526111e0526111c0526111a05261118052611160526111405261112052611100526110e0526110c0526110a0526110805261106052611040526110205261100052610fe052610fc052610fa052610f8052610f6052610f4052610f2052610f0052610ee052610ec052610ea052610e8052610e6052610e4052610e2052610e0052610de052610dc052610da052610d8052610d6052610d4052610d2052610d0052610ce052610cc052610ca052610c8052610c6052610c4052610c2052610c0052610be052610bc052610ba052610b8052610b6052610b4052610b2052610b0052610ae052610ac052610aa052610a8052610a6052610a4052610a2052610a00526109e0526109c0526109a05261098052610960526109405261092052610900526108e0526108c0526108a05261088052610860526108405261082052610800526107e0526107c0526107a05261078052610760526107405261072052610700526106e0526106c0526106a05261068052610660526106405261062052610600526105e0526105c0526105a05261058052610560526105405261052052610500526104e0526104c0526104a05261048052610460526104405261042052610400526103e0526103c0526103a05261038052610360526103405261032052610300526102e0526102c0526102a05261028052610260526102405261022052610200526101e0526101c0526101a0526101805261016052611dc0805160200180611ce0828460006004600a8704601201f161145657600080fd5b50506001600160e05260c052604060c02054611ea0526060611e6052611e6051611ec052610a00805160200180611e6051611ea001828460006004600a8704601201f16114a257600080fd5b5050611e6051611ea001611e408151610820818352015b610820611e40511015156114cc576114e9565b6000611e40516020850101535b81516001018083528114156114b9575b5050506020611e6051611ea0015160206001820306601f8201039050611e60510101611e6052611e6051611ee052611ce0805160200180611e6051611ea001828460006004600a8704601201f161153f57600080fd5b5050611e6051611ea001611e4081516020818352015b6020611e405110151561156757611584565b6000611e40516020850101535b8151600101808352811415611555575b5050506020611e6051611ea0015160206001820306601f8201039050611e60510101611e60527ffef24b0e170d72eb566899dc3a6d4396d901ceb46442d0b04f22e5fc8ec3c611611e6051611ea0a1610a008051602082012090506001611cc05160e05260c052604060c02055611f0060006020818352015b611cc0600261160b57600080fd5b600281510481525060006001611cc0511515611628576000611648565b6002611cc0516002611cc05102041461164057600080fd5b6002611cc051025b60e05260c052604060c02054602082611f200101526020810190506001611cc0511515611676576000611696565b6002611cc0516002611cc05102041461168e57600080fd5b6002611cc051025b6001611cc05115156116a95760006116c9565b6002611cc0516002611cc0510204146116c157600080fd5b6002611cc051025b0110156116d557600080fd5b6001611cc05115156116e8576000611708565b6002611cc0516002611cc05102041461170057600080fd5b6002611cc051025b0160e05260c052604060c02054602082611f2001015260208101905080611f2052611f2090508051602082012090506001611cc05160e05260c052604060c020555b81516001018083528114156115fd575b5050600280546001825401101561177057600080fd5b60018154018155506407735940006109a051141561218e57600380546001825401101561179c57600080fd5b6001815401815550600054600354141561218d57620151806117bd57600080fd5b620151806109e051066109e05110156117d557600080fd5b620151806117e257600080fd5b620151806109e051066109e05103620151806201518061180157600080fd5b620151806109e051066109e051101561181957600080fd5b6201518061182657600080fd5b620151806109e051066109e0510301101561184057600080fd5b620151806201518061185157600080fd5b620151806109e051066109e051101561186957600080fd5b6201518061187657600080fd5b620151806109e051066109e0510301611fa0526001600160e05260c052604060c02054612020526040611fe052611fe0516120405261016051610180516101a0516101c0516101e05161020051610220516102405161026051610280516102a0516102c0516102e05161030051610320516103405161036051610380516103a0516103c0516103e05161040051610420516104405161046051610480516104a0516104c0516104e05161050051610520516105405161056051610580516105a0516105c0516105e05161060051610620516106405161066051610680516106a0516106c0516106e05161070051610720516107405161076051610780516107a0516107c0516107e05161080051610820516108405161086051610880516108a0516108c0516108e05161090051610920516109405161096051610980516109a0516109c0516109e051610a0051610a2051610a4051610a6051610a8051610aa051610ac051610ae051610b0051610b2051610b4051610b6051610b8051610ba051610bc051610be051610c0051610c2051610c4051610c6051610c8051610ca051610cc051610ce051610d0051610d2051610d4051610d6051610d8051610da051610dc051610de051610e0051610e2051610e4051610e6051610e8051610ea051610ec051610ee051610f0051610f2051610f4051610f6051610f8051610fa051610fc051610fe05161100051611020516110405161106051611080516110a0516110c0516110e05161110051611120516111405161116051611180516111a0516111c0516111e05161120051611220516112405161126051611280516112a0516112c0516112e05161130051611320516113405161136051611380516113a0516113c0516113e05161140051611420516114405161146051611480516114a0516114c0516114e05161150051611520516115405161156051611580516115a0516115c0516115e05161160051611620516116405161166051611680516116a0516116c0516116e05161170051611720516117405161176051611780516117a0516117c0516117e05161180051611820516118405161186051611880516118a0516118c0516118e05161190051611920516119405161196051611980516119a0516119c0516119e051611a0051611a2051611a4051611a6051611a8051611aa051611ac051611ae051611b0051611b2051611b4051611b6051611b8051611ba051611bc051611be051611c0051611c2051611c4051611c6051611c8051611ca051611cc051611ce051611d0051611d2051611d4051611d6051611d8051611da051611dc051611de051611e0051611e2051611e4051611e6051611e8051611ea051611ec051611ee051611f0051611f2051611f4051611f6051611f8051611fa051611fc051611fe05161200051612020516120405163ebe0019761206052611fa05161208052612080516006580161009b565b506120e0526000612140525b6120e05160206001820306601f820103905061214051101515611cd757611cf0565b6121405161210001526121405160200161214052611cb5565b612040526120205261200052611fe052611fc052611fa052611f8052611f6052611f4052611f2052611f0052611ee052611ec052611ea052611e8052611e6052611e4052611e2052611e0052611de052611dc052611da052611d8052611d6052611d4052611d2052611d0052611ce052611cc052611ca052611c8052611c6052611c4052611c2052611c0052611be052611bc052611ba052611b8052611b6052611b4052611b2052611b0052611ae052611ac052611aa052611a8052611a6052611a4052611a2052611a00526119e0526119c0526119a05261198052611960526119405261192052611900526118e0526118c0526118a05261188052611860526118405261182052611800526117e0526117c0526117a05261178052611760526117405261172052611700526116e0526116c0526116a05261168052611660526116405261162052611600526115e0526115c0526115a05261158052611560526115405261152052611500526114e0526114c0526114a05261148052611460526114405261142052611400526113e0526113c0526113a05261138052611360526113405261132052611300526112e0526112c0526112a05261128052611260526112405261122052611200526111e0526111c0526111a05261118052611160526111405261112052611100526110e0526110c0526110a0526110805261106052611040526110205261100052610fe052610fc052610fa052610f8052610f6052610f4052610f2052610f0052610ee052610ec052610ea052610e8052610e6052610e4052610e2052610e0052610de052610dc052610da052610d8052610d6052610d4052610d2052610d0052610ce052610cc052610ca052610c8052610c6052610c4052610c2052610c0052610be052610bc052610ba052610b8052610b6052610b4052610b2052610b0052610ae052610ac052610aa052610a8052610a6052610a4052610a2052610a00526109e0526109c0526109a05261098052610960526109405261092052610900526108e0526108c0526108a05261088052610860526108405261082052610800526107e0526107c0526107a05261078052610760526107405261072052610700526106e0526106c0526106a05261068052610660526106405261062052610600526105e0526105c0526105a05261058052610560526105405261052052610500526104e0526104c0526104a05261048052610460526104405261042052610400526103e0526103c0526103a05261038052610360526103405261032052610300526102e0526102c0526102a05261028052610260526102405261022052610200526101e0526101c0526101a05261018052610160526120e0805160200180611fe05161202001828460006004600a8704601201f16120f857600080fd5b5050611fe05161202001611fc081516020818352015b6020611fc0511015156121205761213d565b6000611fc0516020850101535b815160010180835281141561210e575b5050506020611fe051612020015160206001820306601f8201039050611fe0510101611fe0527fd1faa3f9bca1d698df559716fe6d1c9999155b38d3158fffbc98d76d568091fc611fe051612020a15b5b005b63118e4575600051141561224657602060046101403734156121b157600080fd5b61014051640100000000610140510110156121cb57600080fd5b64010000000061014051016105605261058060006020818352015b60016001610560511860e05260c052604060c02054610160610580516020811061220f57600080fd5b6020020152610560600261222257600080fd5b60028151048152505b81516001018083528114156121e6575b5050610400610160f3005b60006000fd5b6100b36122ff036100b36000396100b36122ff036000f3`

// DeployDepositContract deploys a new Ethereum contract, binding an instance of DepositContract to it.
func DeployDepositContract(auth *bind.TransactOpts, backend bind.ContractBackend, depositThreshold *big.Int) (common.Address, *types.Transaction, *DepositContract, error) {
	parsed, err := abi.JSON(strings.NewReader(DepositContractABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(DepositContractBin), backend, depositThreshold)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &DepositContract{DepositContractCaller: DepositContractCaller{contract: contract}, DepositContractTransactor: DepositContractTransactor{contract: contract}, DepositContractFilterer: DepositContractFilterer{contract: contract}}, nil
}

// DepositContract is an auto generated Go binding around an Ethereum contract.
type DepositContract struct {
	DepositContractCaller     // Read-only binding to the contract
	DepositContractTransactor // Write-only binding to the contract
	DepositContractFilterer   // Log filterer for contract events
}

// DepositContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type DepositContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DepositContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type DepositContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DepositContractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type DepositContractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DepositContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type DepositContractSession struct {
	Contract     *DepositContract  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// DepositContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type DepositContractCallerSession struct {
	Contract *DepositContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// DepositContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type DepositContractTransactorSession struct {
	Contract     *DepositContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// DepositContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type DepositContractRaw struct {
	Contract *DepositContract // Generic contract binding to access the raw methods on
}

// DepositContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type DepositContractCallerRaw struct {
	Contract *DepositContractCaller // Generic read-only contract binding to access the raw methods on
}

// DepositContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type DepositContractTransactorRaw struct {
	Contract *DepositContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewDepositContract creates a new instance of DepositContract, bound to a specific deployed contract.
func NewDepositContract(address common.Address, backend bind.ContractBackend) (*DepositContract, error) {
	contract, err := bindDepositContract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &DepositContract{DepositContractCaller: DepositContractCaller{contract: contract}, DepositContractTransactor: DepositContractTransactor{contract: contract}, DepositContractFilterer: DepositContractFilterer{contract: contract}}, nil
}

// NewDepositContractCaller creates a new read-only instance of DepositContract, bound to a specific deployed contract.
func NewDepositContractCaller(address common.Address, caller bind.ContractCaller) (*DepositContractCaller, error) {
	contract, err := bindDepositContract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &DepositContractCaller{contract: contract}, nil
}

// NewDepositContractTransactor creates a new write-only instance of DepositContract, bound to a specific deployed contract.
func NewDepositContractTransactor(address common.Address, transactor bind.ContractTransactor) (*DepositContractTransactor, error) {
	contract, err := bindDepositContract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &DepositContractTransactor{contract: contract}, nil
}

// NewDepositContractFilterer creates a new log filterer instance of DepositContract, bound to a specific deployed contract.
func NewDepositContractFilterer(address common.Address, filterer bind.ContractFilterer) (*DepositContractFilterer, error) {
	contract, err := bindDepositContract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &DepositContractFilterer{contract: contract}, nil
}

// bindDepositContract binds a generic wrapper to an already deployed contract.
func bindDepositContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(DepositContractABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DepositContract *DepositContractRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _DepositContract.Contract.DepositContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DepositContract *DepositContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DepositContract.Contract.DepositContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DepositContract *DepositContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DepositContract.Contract.DepositContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DepositContract *DepositContractCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _DepositContract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DepositContract *DepositContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DepositContract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DepositContract *DepositContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DepositContract.Contract.contract.Transact(opts, method, params...)
}

// GetBranch is a free data retrieval call binding the contract method 0x118e4575.
//
// Solidity: function get_branch(leaf uint256) constant returns(out bytes32[32])
func (_DepositContract *DepositContractCaller) GetBranch(opts *bind.CallOpts, leaf *big.Int) ([32][32]byte, error) {
	var (
		ret0 = new([32][32]byte)
	)
	out := ret0
	err := _DepositContract.contract.Call(opts, out, "get_branch", leaf)
	return *ret0, err
}

// GetBranch is a free data retrieval call binding the contract method 0x118e4575.
//
// Solidity: function get_branch(leaf uint256) constant returns(out bytes32[32])
func (_DepositContract *DepositContractSession) GetBranch(leaf *big.Int) ([32][32]byte, error) {
	return _DepositContract.Contract.GetBranch(&_DepositContract.CallOpts, leaf)
}

// GetBranch is a free data retrieval call binding the contract method 0x118e4575.
//
// Solidity: function get_branch(leaf uint256) constant returns(out bytes32[32])
func (_DepositContract *DepositContractCallerSession) GetBranch(leaf *big.Int) ([32][32]byte, error) {
	return _DepositContract.Contract.GetBranch(&_DepositContract.CallOpts, leaf)
}

// GetDepositRoot is a free data retrieval call binding the contract method 0xc5f2892f.
//
// Solidity: function get_deposit_root() constant returns(out bytes32)
func (_DepositContract *DepositContractCaller) GetDepositRoot(opts *bind.CallOpts) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _DepositContract.contract.Call(opts, out, "get_deposit_root")
	return *ret0, err
}

// GetDepositRoot is a free data retrieval call binding the contract method 0xc5f2892f.
//
// Solidity: function get_deposit_root() constant returns(out bytes32)
func (_DepositContract *DepositContractSession) GetDepositRoot() ([32]byte, error) {
	return _DepositContract.Contract.GetDepositRoot(&_DepositContract.CallOpts)
}

// GetDepositRoot is a free data retrieval call binding the contract method 0xc5f2892f.
//
// Solidity: function get_deposit_root() constant returns(out bytes32)
func (_DepositContract *DepositContractCallerSession) GetDepositRoot() ([32]byte, error) {
	return _DepositContract.Contract.GetDepositRoot(&_DepositContract.CallOpts)
}

// Deposit is a paid mutator transaction binding the contract method 0x98b1e06a.
//
// Solidity: function deposit(deposit_input bytes) returns()
func (_DepositContract *DepositContractTransactor) Deposit(opts *bind.TransactOpts, deposit_input []byte) (*types.Transaction, error) {
	return _DepositContract.contract.Transact(opts, "deposit", deposit_input)
}

// Deposit is a paid mutator transaction binding the contract method 0x98b1e06a.
//
// Solidity: function deposit(deposit_input bytes) returns()
func (_DepositContract *DepositContractSession) Deposit(deposit_input []byte) (*types.Transaction, error) {
	return _DepositContract.Contract.Deposit(&_DepositContract.TransactOpts, deposit_input)
}

// Deposit is a paid mutator transaction binding the contract method 0x98b1e06a.
//
// Solidity: function deposit(deposit_input bytes) returns()
func (_DepositContract *DepositContractTransactorSession) Deposit(deposit_input []byte) (*types.Transaction, error) {
	return _DepositContract.Contract.Deposit(&_DepositContract.TransactOpts, deposit_input)
}

// DepositContractChainStartIterator is returned from FilterChainStart and is used to iterate over the raw logs and unpacked data for ChainStart events raised by the DepositContract contract.
type DepositContractChainStartIterator struct {
	Event *DepositContractChainStart // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *DepositContractChainStartIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DepositContractChainStart)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(DepositContractChainStart)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *DepositContractChainStartIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DepositContractChainStartIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DepositContractChainStart represents a ChainStart event raised by the DepositContract contract.
type DepositContractChainStart struct {
	DepositRoot [32]byte
	Time        []byte
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterChainStart is a free log retrieval operation binding the contract event 0xd1faa3f9bca1d698df559716fe6d1c9999155b38d3158fffbc98d76d568091fc.
//
// Solidity: e ChainStart(deposit_root bytes32, time bytes)
func (_DepositContract *DepositContractFilterer) FilterChainStart(opts *bind.FilterOpts) (*DepositContractChainStartIterator, error) {

	logs, sub, err := _DepositContract.contract.FilterLogs(opts, "ChainStart")
	if err != nil {
		return nil, err
	}
	return &DepositContractChainStartIterator{contract: _DepositContract.contract, event: "ChainStart", logs: logs, sub: sub}, nil
}

// WatchChainStart is a free log subscription operation binding the contract event 0xd1faa3f9bca1d698df559716fe6d1c9999155b38d3158fffbc98d76d568091fc.
//
// Solidity: e ChainStart(deposit_root bytes32, time bytes)
func (_DepositContract *DepositContractFilterer) WatchChainStart(opts *bind.WatchOpts, sink chan<- *DepositContractChainStart) (event.Subscription, error) {

	logs, sub, err := _DepositContract.contract.WatchLogs(opts, "ChainStart")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DepositContractChainStart)
				if err := _DepositContract.contract.UnpackLog(event, "ChainStart", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// DepositContractDepositIterator is returned from FilterDeposit and is used to iterate over the raw logs and unpacked data for Deposit events raised by the DepositContract contract.
type DepositContractDepositIterator struct {
	Event *DepositContractDeposit // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *DepositContractDepositIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DepositContractDeposit)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(DepositContractDeposit)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *DepositContractDepositIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DepositContractDepositIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DepositContractDeposit represents a Deposit event raised by the DepositContract contract.
type DepositContractDeposit struct {
	PreviousDepositRoot [32]byte
	Data                []byte
	MerkleTreeIndex     []byte
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterDeposit is a free log retrieval operation binding the contract event 0xfef24b0e170d72eb566899dc3a6d4396d901ceb46442d0b04f22e5fc8ec3c611.
//
// Solidity: e Deposit(previous_deposit_root bytes32, data bytes, merkle_tree_index bytes)
func (_DepositContract *DepositContractFilterer) FilterDeposit(opts *bind.FilterOpts) (*DepositContractDepositIterator, error) {

	logs, sub, err := _DepositContract.contract.FilterLogs(opts, "Deposit")
	if err != nil {
		return nil, err
	}
	return &DepositContractDepositIterator{contract: _DepositContract.contract, event: "Deposit", logs: logs, sub: sub}, nil
}

// WatchDeposit is a free log subscription operation binding the contract event 0xfef24b0e170d72eb566899dc3a6d4396d901ceb46442d0b04f22e5fc8ec3c611.
//
// Solidity: e Deposit(previous_deposit_root bytes32, data bytes, merkle_tree_index bytes)
func (_DepositContract *DepositContractFilterer) WatchDeposit(opts *bind.WatchOpts, sink chan<- *DepositContractDeposit) (event.Subscription, error) {

	logs, sub, err := _DepositContract.contract.WatchLogs(opts, "Deposit")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DepositContractDeposit)
				if err := _DepositContract.contract.UnpackLog(event, "Deposit", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}
