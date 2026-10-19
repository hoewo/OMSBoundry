package httpserver

import (
	"os/exec"
	"strings"
	"testing"
)

func TestEvidenceDemoUsesSupportedDirectDeliveryMode(t *testing.T) {
	if !strings.Contains(evidenceDemoJavaScript, "delivery_mode:'direct'") {
		t.Fatal("evidence demo download must request the versioned contract's direct delivery mode")
	}
	if strings.Contains(evidenceDemoJavaScript, "delivery_mode:'redirect'") {
		t.Fatal("evidence demo must not send the unsupported redirect delivery mode")
	}
}

func TestEvidencePageJavaScriptSyntaxAndCNameAddressing(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is unavailable")
	}
	check := exec.Command("node", "--check")
	check.Stdin = strings.NewReader(evidenceDemoJavaScript)
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("page script syntax: %v %s", err, output)
	}
	start := strings.Index(evidenceDemoJavaScript, "function objectURL(a)")
	end := strings.Index(evidenceDemoJavaScript, "async function putOSS")
	if start < 0 || end < start {
		t.Fatal("missing object addressing function")
	}
	code := evidenceDemoJavaScript[start:end] + `;for(const cname of [true,false]) { const u=objectURL({endpoint:'https://files.example.test',bucket:'bucket',object_key:'tenants/t/file name',endpoint_is_cname:cname}); const expected='https://'+(cname?'':'bucket.')+'files.example.test/tenants/t/file%20name';if(u!==expected)process.exit(1);}`
	if err := exec.Command("node", "-e", code).Run(); err != nil {
		t.Fatal("CNAME page addressing failed")
	}
}

func TestEvidencePageOSSDateIsIncludedInSignature(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is unavailable")
	}
	start := strings.Index(evidenceDemoJavaScript, "function objectURL(a)")
	end := strings.Index(evidenceDemoJavaScript, "async function upload(")
	if start < 0 || end < start {
		t.Fatal("missing OSS upload functions")
	}
	code := evidenceDemoJavaScript[start:end] + `;let signed;async function hmacSHA1(_key,text){signed=text;return 'sig'};async function fetch(_url,opt){if(signed.split('\n')[3]!==opt.headers['x-oss-date'])throw new Error('Date omitted from V1 signature');return {ok:true,headers:{get:()=> 'etag'}}};putOSS({endpoint:'https://files.example.test',endpoint_is_cname:true,bucket:'bucket',object_key:'tenants/t/file',security_token:'fixture-token',access_key_id:'fixture-id',access_key_secret:'fixture-secret'},new Blob(['x'],{type:'text/plain'})).catch(e=>{console.error(e.message);process.exit(1)});`
	if out, err := exec.Command("node", "-e", code).CombinedOutput(); err != nil {
		t.Fatalf("OSS page date signature: %v %s", err, out)
	}
}
