/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/13 9:06 上午
 * @Desc: TODO
 */

package tcp

import "testing"

func Test_Pack(t *testing.T) {
	msg := []byte("hello world")

	packet, err := pack(msg)
	if err != nil {
		t.Fatal(err)
	}

	msg, err = unpack(packet)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(msg))
}
