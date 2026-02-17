package web

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func AdminPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

const html = `
<!DOCTYPE html>
<html>
<head>
    <title>Emby2Alist Go</title>
    <meta charset="utf-8">
    <link href="https://unpkg.com/element-plus/dist/index.css" rel="stylesheet">
    <script src="https://unpkg.com/vue@3"></script>
    <script src="https://unpkg.com/element-plus"></script>
    <script src="https://unpkg.com/axios/dist/axios.min.js"></script>
    <style>body{padding:20px;background:#f5f7fa;font-family:sans-serif}.card{max-width:1000px;margin:0 auto;margin-bottom:20px}</style>
</head>
<body>
<div id="app">
    <el-card class="card">
        <template #header>
            <div style="display:flex;justify-content:space-between;align-items:center">
                <h3>Emby2Alist Go 管理后台</h3>
                <el-button type="primary" @click="save" :loading="loading">保存配置</el-button>
            </div>
        </template>
        
        <el-form :model="form" label-width="140px">
            <el-tabs>
                <el-tab-pane label="基础设置">
                    <el-form-item label="后端类型">
                        <el-radio-group v-model="form.backend_type">
                            <el-radio label="emby">Emby</el-radio>
                            <el-radio label="jellyfin">Jellyfin</el-radio>
                        </el-radio-group>
                    </el-form-item>
                    <el-form-item label="媒体服务器地址">
                        <el-input v-model="form.emby_host" placeholder="http://172.17.0.1:8096"></el-input>
                    </el-form-item>
                    <el-form-item label="API Key">
                        <el-input v-model="form.emby_api_key"></el-input>
                    </el-form-item>
                    <el-divider></el-divider>
                    <el-form-item label="Alist 地址">
                        <el-input v-model="form.alist_host" placeholder="http://172.17.0.1:5244"></el-input>
                    </el-form-item>
                    <el-form-item label="Alist 公网地址">
                        <el-input v-model="form.alist_public_host" placeholder="客户端302跳转地址"></el-input>
                    </el-form-item>
                    <el-form-item label="Alist Token">
                        <el-input v-model="form.alist_token"></el-input>
                    </el-form-item>
                    <el-form-item label="启用签名">
                        <el-switch v-model="form.alist_sign_enable"></el-switch>
                    </el-form-item>
                    <el-form-item label="签名 Salt">
                        <el-input v-model="form.alist_sign_salt" placeholder="Alist 后台 Token 页面查看"></el-input>
                    </el-form-item>
                    <el-form-item label="禁用转码">
                         <el-switch v-model="form.disable_transcode"></el-switch>
                         <span style="font-size:12px;color:#999;margin-left:10px">强制客户端直连，不走转码</span>
                    </el-form-item>
                </el-tab-pane>

                <el-tab-pane label="路径映射">
                    <el-form-item label="本地挂载路径">
                        <el-select v-model="form.mount_paths" multiple allow-create filterable default-first-option placeholder="输入路径回车添加"></el-select>
                        <div style="font-size:12px;color:#999">以此开头的路径才会去请求 Alist，Strm 文件不受此限制</div>
                    </el-form-item>
                    <el-table :data="form.path_mappings" border>
                        <el-table-column label="Emby路径(Old)" prop="old">
                            <template #default="{row}"><el-input v-model="row.old"></el-input></template>
                        </el-table-column>
                        <el-table-column label="Alist路径(New)" prop="new">
                            <template #default="{row}"><el-input v-model="row.new"></el-input></template>
                        </el-table-column>
                        <el-table-column width="60" align="center">
                            <template #default="{$index}"><el-button type="danger" icon="Delete" circle @click="form.path_mappings.splice($index,1)"></el-button></template>
                        </el-table-column>
                    </el-table>
                    <el-button style="margin-top:10px" @click="addMap">添加映射</el-button>
                </el-tab-pane>

                <el-tab-pane label="高级规则">
                    <el-alert title="分组相同的规则为 AND 关系，不同分组为 OR 关系。Block 优先级最高。" type="info" style="margin-bottom:10px" :closable="false"></el-alert>
                    <el-table :data="form.route_rules" border>
                        <el-table-column label="分组" width="100" prop="group">
                             <template #default="{row}"><el-input v-model="row.group" placeholder="default"></el-input></template>
                        </el-table-column>
                        <el-table-column label="模式" width="100">
                            <template #default="{row}">
                                <el-select v-model="row.mode">
                                    <el-option label="代理" value="proxy"></el-option>
                                    <el-option label="直连" value="redirect"></el-option>
                                    <el-option label="拦截" value="block"></el-option>
                                </el-select>
                            </template>
                        </el-table-column>
                        <el-table-column label="对象" width="110">
                            <template #default="{row}">
                                <el-select v-model="row.target">
                                    <el-option label="文件路径" value="filePath"></el-option>
                                    <el-option label="User-Agent" value="ua"></el-option>
                                    <el-option label="客户IP" value="remote_addr"></el-option>
                                    <el-option label="用户ID" value="userId"></el-option>
                                </el-select>
                            </template>
                        </el-table-column>
                         <el-table-column label="匹配" width="110">
                            <template #default="{row}">
                                <el-select v-model="row.matcher">
                                    <el-option label="包含" value="contains"></el-option>
                                    <el-option label="前缀" value="startsWith"></el-option>
                                    <el-option label="后缀" value="endsWith"></el-option>
                                    <el-option label="正则" value="regex"></el-option>
                                    <el-option label="相等" value="eq"></el-option>
                                    <el-option label="IP段" value="cidr"></el-option>
                                </el-select>
                            </template>
                        </el-table-column>
                        <el-table-column label="值" prop="value">
                            <template #default="{row}"><el-input v-model="row.value"></el-input></template>
                        </el-table-column>
                         <el-table-column width="60" align="center">
                            <template #default="{$index}"><el-button type="danger" icon="Delete" circle @click="form.route_rules.splice($index,1)"></el-button></template>
                        </el-table-column>
                    </el-table>
                    <el-button style="margin-top:10px" @click="addRule">添加规则</el-button>
                </el-tab-pane>
            </el-tabs>
        </el-form>
    </el-card>
</div>
<script>
    Vue.createApp({
        data() { return { form: {}, loading: false } },
        mounted() { this.load() },
        methods: {
            async load() {
                try {
                    const res = await axios.get('/api/config');
                    this.form = res.data;
                    if(!this.form.path_mappings) this.form.path_mappings=[];
                    if(!this.form.route_rules) this.form.route_rules=[];
                } catch(e) { ElementPlus.ElMessage.error('加载失败'); }
            },
            async save() {
                this.loading = true;
                try {
                    await axios.post('/api/config', this.form);
                    ElementPlus.ElMessage.success('保存成功，即时生效');
                } catch(e) { ElementPlus.ElMessage.error('保存失败'); }
                this.loading = false;
            },
            addMap() { this.form.path_mappings.push({old:'',new:''}) },
            addRule() { this.form.route_rules.push({group:'default',mode:'proxy',target:'ua',matcher:'contains',value:''}) }
        }
    }).use(ElementPlus).mount('#app');
</script>
</body>
</html>
`