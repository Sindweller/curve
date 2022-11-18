/*
 *  Copyright (c) 2022 NetEase Inc.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

/*
 * Project: CurveCli
 * Created Date: 2022-11-17
 * Author: Sindweller
 */

package dir

import (
	"context"
	"fmt"
	cmderror "github.com/opencurve/curve/tools-v2/internal/error"
	cobrautil "github.com/opencurve/curve/tools-v2/internal/utils"
	basecmd "github.com/opencurve/curve/tools-v2/pkg/cli/command"
	"github.com/opencurve/curve/tools-v2/pkg/cli/command/curvebs/query/file"
	"github.com/opencurve/curve/tools-v2/pkg/config"
	"github.com/opencurve/curve/tools-v2/pkg/output"
	"github.com/opencurve/curve/tools-v2/proto/proto/nameserver2"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"log"
)

const (
	dirExample = `$ curve bs list dir -fileName=/test`
)

type ListDirRpc struct {
	Info          *basecmd.Rpc
	Request       *nameserver2.ListDirRequest
	curveFSClient nameserver2.CurveFSServiceClient
}

var _ basecmd.RpcFunc = (*ListDirRpc)(nil) // check interface

type DirCommand struct {
	basecmd.FinalCurveCmd
	Rpc []*ListDirRpc
}

var _ basecmd.FinalCurveCmdFunc = (*DirCommand)(nil) // check interface

func (lRpc *ListDirRpc) NewRpcClient(cc grpc.ClientConnInterface) {
	lRpc.curveFSClient = nameserver2.NewCurveFSServiceClient(cc)
}

func (lRpc *ListDirRpc) Stub_Func(ctx context.Context) (interface{}, error) {
	return lRpc.curveFSClient.ListDir(ctx, lRpc.Request)
}

func NewDirCommand() *cobra.Command {
	return NewListDirCommand().Cmd
}

func NewListDirCommand() *DirCommand {
	lsCmd := &DirCommand{
		FinalCurveCmd: basecmd.FinalCurveCmd{
			Use:     "dir",
			Short:   "list dir information in curvebs",
			Example: dirExample,
		},
	}

	basecmd.NewFinalCurveCli(&lsCmd.FinalCurveCmd, lsCmd)
	return lsCmd
}

// AddFlags implements basecmd.FinalCurveCmdFunc
func (pCmd *DirCommand) AddFlags() {
	config.AddBsMdsFlagOption(pCmd.Cmd)
	config.AddRpcRetryTimesFlag(pCmd.Cmd)
	config.AddRpcTimeoutFlag(pCmd.Cmd)
	config.AddBsDirOptionFlag(pCmd.Cmd)
}

// Init implements basecmd.FinalCurveCmdFunc
func (pCmd *DirCommand) Init(cmd *cobra.Command, args []string) error {
	mdsAddrs, err := config.GetBsMdsAddrSlice(pCmd.Cmd)
	if err.TypeCode() != cmderror.CODE_SUCCESS {
		return err.ToError()
	}

	timeout := config.GetFlagDuration(pCmd.Cmd, config.RPCTIMEOUT)
	retrytimes := config.GetFlagInt32(pCmd.Cmd, config.RPCRETRYTIMES)
	fileName := config.GetBsFlagString(pCmd.Cmd, config.CURVEBS_DIR)

	rpc := &ListDirRpc{
		Request: &nameserver2.ListDirRequest{
			FileName: &fileName,
		},
		Info: basecmd.NewRpc(mdsAddrs, timeout, retrytimes, "ListDir"),
	}
	pCmd.Rpc = append(pCmd.Rpc, rpc)
	header := []string{
		cobrautil.ROW_FILE_NAME,
		cobrautil.ROW_PARENT_ID,
		cobrautil.ROW_FILE_TYPE,
		cobrautil.ROW_OWNER,
		cobrautil.ROW_CTIME,
		cobrautil.ROW_ALLOC_SIZE,
		cobrautil.ROW_FILE_SIZE,
	}
	pCmd.SetHeader(header)
	pCmd.TableNew.SetAutoMergeCellsByColumnIndex(cobrautil.GetIndexSlice(
		pCmd.Header, header,
	))
	return nil
}

// Print implements basecmd.FinalCurveCmdFunc
func (pCmd *DirCommand) Print(cmd *cobra.Command, args []string) error {
	return output.FinalCmdOutput(&pCmd.FinalCurveCmd, pCmd)
}

// RunCommand implements basecmd.FinalCurveCmdFunc
func (pCmd *DirCommand) RunCommand(cmd *cobra.Command, args []string) error {
	var infos []*basecmd.Rpc
	var funcs []basecmd.RpcFunc
	for _, rpc := range pCmd.Rpc {
		infos = append(infos, rpc.Info)
		funcs = append(funcs, rpc)
	}
	results, errs := basecmd.GetRpcListResponse(infos, funcs)
	if len(errs) == len(infos) {
		mergeErr := cmderror.MergeCmdErrorExceptSuccess(errs)
		return mergeErr.ToError()
	}
	var errors []*cmderror.CmdError
	rows := make([]map[string]string, 0)
	for _, res := range results {
		infos := res.(*nameserver2.ListDirResponse).GetFileInfo()
		for _, info := range infos {
			fmt.Println(info)
			row := make(map[string]string)
			dirName := config.GetBsFlagString(pCmd.Cmd, config.CURVEBS_DIR)
			if dirName == "/" {
				row[cobrautil.ROW_FILE_NAME] = dirName + info.GetFileName()
			} else {
				row[cobrautil.ROW_FILE_NAME] = dirName + "/" + info.GetFileName()
			}
			row[cobrautil.ROW_PARENT_ID] = string(info.GetParentId())
			row[cobrautil.ROW_FILE_TYPE] = string(info.GetFileType())
			row[cobrautil.ROW_OWNER] = info.GetOwner()
			row[cobrautil.ROW_CTIME] = string(info.GetCtime())
			// Get file size
			sizeRes, err := file.GetFileSize(pCmd.Cmd)
			if err.TypeCode() != cmderror.CODE_SUCCESS {
				log.Printf("%s failed to get file size: %v", info.GetFileName(), err)
			}
			row[cobrautil.ROW_FILE_SIZE] = string(sizeRes.GetFileSize())
			// Get allocated size
			allocRes, err := file.GetAllocatedSize(pCmd.Cmd)
			if err.TypeCode() != cmderror.CODE_SUCCESS {
				log.Printf("%s failed to get allocated size: %v", info.GetFileName(), err)
			}
			row[cobrautil.ROW_ALLOC_SIZE] = string(allocRes.GetAllocatedSize())
			rows = append(rows, row)
		}
	}
	list := cobrautil.ListMap2ListSortByKeys(rows, pCmd.Header, []string{
		cobrautil.ROW_FILE_NAME,
	})
	pCmd.TableNew.AppendBulk(list)
	errRet := cmderror.MergeCmdError(errors)
	pCmd.Error = &errRet
	pCmd.Result = results
	return nil
}

// ResultPlainOutput implements basecmd.FinalCurveCmdFunc
func (pCmd *DirCommand) ResultPlainOutput() error {
	return output.FinalCmdOutputPlain(&pCmd.FinalCurveCmd)
}
