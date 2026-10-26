"use strict";
// Actual-package acceptance, Linux only; no daily client directories touched.
const fs=require("node:fs"),os=require("node:os"),path=require("node:path"),assert=require("node:assert/strict");
const {spawnSync}=require("node:child_process");
if(process.platform!=="linux") throw Error("Use isolated Linux for npm postinstall acceptance");
const [pkg,officialRoot,evidence]=process.argv.slice(2);
const root=fs.mkdtempSync(path.join(os.tmpdir(),"cli-history-provenance-"));
fs.mkdirSync(evidence,{recursive:true});
const report={root,cases:[],notIncluded:["Windows npm installation","macOS","AI client loading"]};
const npm=path.resolve(path.dirname(process.execPath),"../lib/node_modules/npm/bin/npm-cli.js");
let counter=0;
function run(command,args,env){
 const r=spawnSync(command,args,{env,encoding:"utf8",timeout:120000,maxBuffer:16*1024*1024});
 fs.writeFileSync(path.join(evidence,`${++counter}.log`),JSON.stringify({command,args,status:r.status,stdout:r.stdout,stderr:r.stderr},null,2));
 assert.equal(r.status,0,r.stderr||String(r.error));return r.stdout+r.stderr;
}
try {
 for(const scenario of ["official-1.9.3","official-1.9.4","modified-history","third-party-auth","custom-directory"]){
  const base=path.join(root,scenario);fs.mkdirSync(base,{recursive:true});
  const env={...process.env,PATH:path.dirname(process.execPath)+":"+process.env.PATH,CONTRACT_CLI_CONFIG_DIR:path.join(base,"config"),CODEX_HOME:path.join(base,"codex"),CONTRACT_CLI_NO_UPDATE_CHECK:"1",npm_config_cache:path.join(root,"cache"),npm_config_update_notifier:"false"};
  const target=path.join(scenario==="custom-directory"?path.join(base,"custom"):env.CODEX_HOME,"skills");
  const prefix=path.join(base,"npm");const entry=path.join(prefix,"lib/node_modules/@qfeius/contract-cli/scripts/run.js");
  const registry=path.join(env.CONTRACT_CLI_CONFIG_DIR,"skill-locations.json");
  if(scenario!=="third-party-auth"){
   const version=scenario==="official-1.9.4"?"1.9.4":"1.9.3";
   run("tar",["-xzf",path.join(officialRoot,version,"package/dist/release-assets",`contract-cli-${version}-linux-amd64.tar.gz`),"-C",base],env);
   run(path.join(base,"contract-cli"),["skills","install","--target",target,"--force"],env);
  } else fs.mkdirSync(path.join(target,"auth"),{recursive:true});
  const skill=path.join(target,"auth/SKILL.md");
  const unverified=["modified-history","third-party-auth"].includes(scenario);
  if(unverified)fs.writeFileSync(skill,"unverified-content-must-stay");
  const before=fs.readFileSync(skill);
  const install=()=>run(process.execPath,[npm,"install","-g",pkg,"--prefix",prefix,"--foreground-scripts","--no-audit","--no-fund"],env);
  const output=install();
  if(scenario.startsWith("official-")){
   assert.match(output,/Skill 自动同步：成功/);
   assert.match(fs.readFileSync(skill,"utf8"),/version: 1.9.4-test.clm111796.21/);
   assert.equal(JSON.parse(fs.readFileSync(registry)).locations.length,1);
  } else {
   if(unverified)assert.match(output,/历史 Skill 未自动接管/);
   assert.deepEqual(fs.readFileSync(skill),before);assert.equal(fs.existsSync(registry),false);
   run(process.execPath,[entry,"skills","install","--target",target,"--force"],env);
   run(process.execPath,[entry,"skills","install","--target",target,"--force"],env);
   assert.equal(JSON.parse(fs.readFileSync(registry)).locations.length,1);
   fs.writeFileSync(skill,"registered-user-modification");
   assert.match(install(),/Skill 自动同步：成功/);
   assert.match(fs.readFileSync(skill,"utf8"),/version: 1.9.4-test.clm111796.21/);
  }
  report.cases.push({name:scenario,status:"PASS"});console.log("PASS "+scenario);
 }
 report.status="PASS";
}catch(error){report.status="FAIL";report.error=error.message;process.exitCode=1;console.error(error)}
finally{fs.writeFileSync(path.join(evidence,"report.json"),JSON.stringify(report,null,2));}
