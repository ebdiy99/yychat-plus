<template>
  <van-config-provider :theme="getMobileTheme()">
    <div class="mobile-chat">
      <van-sticky ref="navBarRef" :offset-top="0" position="top">
        <van-nav-bar left-arrow left-text="返回" @click-left="router.back()">
          <template #title>
            <van-dropdown-menu>
              <van-dropdown-item :title="title">
                <van-cell center title="角色"> {{ role.name }}</van-cell>
                <van-cell center title="模型">{{ model }}</van-cell>
              </van-dropdown-item>
            </van-dropdown-menu>
          </template>

          <template #right>
            <van-icon name="share-o" @click="showShare = true"/>
          </template>
        </van-nav-bar>
      </van-sticky>

      <van-share-sheet
          v-model:show="showShare"
          title="立即分享给好友"
          :options="shareOptions"
          @select="shareChat"
      />

      <div id="message-list-box" :style="{height: winHeight+'px'}" class="message-list-box">
        <van-list
            v-model:error="error"
            v-model:loading="loading"
            :finished="finished"
            error-text="请求失败，点击重新加载"
            @load="onLoad"
        >
          <van-cell v-for="item in chatData" :key="item" :border="false" class="message-line">
            <chat-prompt
                v-if="item.type==='prompt'"
                :content="item.content"
                :created-at="dateFormat(item['created_at'])"
                :icon="item.icon"
                :model="model"
                :tokens="item['tokens']"/>
            <chat-reply v-else-if="item.type==='reply'"
                        :content="item.content"
                        :created-at="dateFormat(item['created_at'])"
                        :icon="item.icon"
                        :org-content="item.orgContent"
                        :tokens="item['tokens']"/>
          </van-cell>
        </van-list>
      </div>

      <van-sticky ref="bottomBarRef" :offset-bottom="0" position="bottom">
        <div class="chat-box">
          <van-cell-group>
            <van-field
                v-model="prompt"
                center
                clearable
                placeholder="输入你的问题"
            >
              <template #button>
                <van-button size="small" type="primary" @click="sendMessage">发送</van-button>
              </template>
              <template #extra>
                <div class="icon-box">
                  <van-icon v-if="showStopGenerate" name="stop-circle-o" @click="stopGenerate"/>
                  <van-icon v-if="showReGenerate" name="play-circle-o" @click="reGenerate"/>
                </div>
              </template>
            </van-field>
          </van-cell-group>
        </div>
      </van-sticky>
    </div>
  </van-config-provider>

</template>

<script setup>
import {nextTick, onMounted, ref} from "vue";
import {showToast, showDialog} from "vant";
import {useRouter} from "vue-router";
import {dateFormat, randString, renderInputText, UUID} from "@/utils/libs";
import {getChatConfig} from "@/store/chat";
import {httpGet} from "@/utils/http";
import hl from "highlight.js";
import 'highlight.js/styles/a11y-dark.css'
import ChatPrompt from "@/components/mobile/ChatPrompt.vue";
import ChatReply from "@/components/mobile/ChatReply.vue";
import {getSessionId} from "@/store/session";
import {checkSession} from "@/action/session";
import {getMobileTheme} from "@/store/system";

const winHeight = ref(0)
const navBarRef = ref(null)
const bottomBarRef = ref(null)
const router = useRouter()

const chatConfig = getChatConfig()
const role = chatConfig.role
const model = chatConfig.model
const title = chatConfig.title
const chatId = chatConfig.chatId
const loginUser = ref(null)

onMounted(() => {
  winHeight.value = document.body.offsetHeight - navBarRef.value.$el.offsetHeight - bottomBarRef.value.$el.offsetHeight
})

const chatData = ref([])
const loading = ref(false)
const finished = ref(false)
const error = ref(false)

checkSession().then(user => {
  loginUser.value = user
}).catch(() => {
  router.push('/login')
})

const onLoad = () => {
  httpGet('/api/chat/history?chat_id=' + chatId).then(res => {
    // 加载状态结束
    loading.value = false;
    finished.value = true;
    const data = res.data
    if (data && data.length > 0) {
      const md = require('markdown-it')({breaks: true});
      for (let i = 0; i < data.length; i++) {
        if (data[i].type === "prompt") {
          chatData.value.push(data[i]);
          continue;
        }

        data[i].orgContent = data[i].content;
        data[i].content = md.render(data[i].content);
        chatData.value.push(data[i]);
      }

      nextTick(() => {
        hl.configure({ignoreUnescapedHTML: true})
        const blocks = document.querySelector("#message-list-box").querySelectorAll('pre code');
        blocks.forEach((block) => {
          hl.highlightElement(block)
        })

        scrollListBox()
      })
    }

    // 连接会话
    connect(chatId, role.id);
  }).catch(() => {
    error.value = true
  })

};

// 创建 socket 连接
const prompt = ref('');
const showStopGenerate = ref(false); // 停止生成
const showReGenerate = ref(false); // 重新生成
const previousText = ref(''); // 上一次提问
const lineBuffer = ref(''); // 输出缓冲行
const socket = ref(null);
const activelyClose = ref(false); // 主动关闭
const canSend = ref(true);
const connect = function (chat_id, role_id) {
  let isNewChat = false;
  if (!chat_id) {
    isNewChat = true;
    chat_id = UUID();
  }
  // 先关闭已有连接
  if (socket.value !== null) {
    activelyClose.value = true;
    socket.value.close();
  }

  // 初始化 WebSocket 对象
  const _sessionId = getSessionId();
  let host = process.env.VUE_APP_WS_HOST
  if (host === '') {
    if (location.protocol === 'https:') {
      host = 'wss://' + location.host;
    } else {
      host = 'ws://' + location.host;
    }
  }
  const _socket = new WebSocket(host + `/api/chat/new?session_id=${_sessionId}&role_id=${role_id}&chat_id=${chat_id}&model=${model}`);
  _socket.addEventListener('open', () => {
    previousText.value = '';
    canSend.value = true;
    activelyClose.value = false;

    if (isNewChat) { // 加载打招呼信息
      loading.value = false;
      chatData.value.push({
        type: "reply",
        id: randString(32),
        icon: role.icon,
        content: role.helloMsg,
        orgContent: role.helloMsg,
      })
    }
  });

  _socket.addEventListener('message', event => {
    if (event.data instanceof Blob) {
      const reader = new FileReader();
      reader.readAsText(event.data, "UTF-8");
      reader.onload = () => {
        const data = JSON.parse(String(reader.result));
        if (data.type === 'start') {
          chatData.value.push({
            type: "reply",
            id: randString(32),
            icon: role.icon,
            content: ""
          });
        } else if (data.type === 'end') { // 消息接收完毕
          canSend.value = true;
          showReGenerate.value = true;
          showStopGenerate.value = false;
          lineBuffer.value = ''; // 清空缓冲

        } else {
          lineBuffer.value += data.content;
          const md = require('markdown-it')({breaks: true});
          const reply = chatData.value[chatData.value.length - 1]
          reply['orgContent'] = lineBuffer.value;
          reply['content'] = md.render(lineBuffer.value);

          nextTick(() => {
            hl.configure({ignoreUnescapedHTML: true})
            const lines = document.querySelectorAll('.message-line');
            const blocks = lines[lines.length - 1].querySelectorAll('pre code');
            blocks.forEach((block) => {
              hl.highlightElement(block)
            })
            scrollListBox()
          })
        }

      };
    }

  });

  _socket.addEventListener('close', () => {
    console.log(activelyClose.value)
    if (activelyClose.value) { // 忽略主动关闭
      return;
    }
    // 停止发送消息
    canSend.value = true;
    socket.value = null;
    checkSession().then(() => {
      connect(chat_id, role_id)
    }).catch(() => {
      showDialog({
        title: '会话提示',
        message: '当前会话已经失效，请重新登录！',
      }).then(() => {
        router.push('/login')
      });
    });
  });

  socket.value = _socket;
}

// 将聊天框的滚动条滑动到最底部
const scrollListBox = () => {
  document.getElementById('message-list-box').scrollTo(0, document.getElementById('message-list-box').scrollHeight + 46)
}

const sendMessage = () => {
  if (canSend.value === false) {
    showToast("AI 正在作答中，请稍后...");
    return
  }

  if (prompt.value.trim().length === 0) {
    return false;
  }

  // 追加消息
  chatData.value.push({
    type: "prompt",
    id: randString(32),
    icon: loginUser.value.avatar,
    content: renderInputText(prompt.value),
    created_at: new Date().getTime(),
  });

  nextTick(() => {
    scrollListBox()
  })

  canSend.value = false;
  showStopGenerate.value = true;
  showReGenerate.value = false;
  socket.value.send(prompt.value);
  previousText.value = prompt.value;
  prompt.value = '';
  return true;
}

const stopGenerate = () => {
  showStopGenerate.value = false;
  httpGet("/api/chat/stop?session_id=" + getSessionId()).then(() => {
    canSend.value = true;
    if (previousText.value !== '') {
      showReGenerate.value = true;
    }
  })
}

const reGenerate = () => {
  canSend.value = false;
  showStopGenerate.value = true;
  showReGenerate.value = false;
  const text = '重新生成上述问题的答案：' + previousText.value;
  // 追加消息
  chatData.value.push({
    type: "prompt",
    id: randString(32),
    icon: loginUser.value.avatar,
    content: renderInputText(text)
  });
  socket.value.send(text);
}

const showShare = ref(false)
const shareOptions = [
  {name: '微信', icon: 'wechat'},
  {name: '微博', icon: 'weibo'},
  {name: '复制链接', icon: 'link'},
  {name: '分享海报', icon: 'poster'},
]
const shareChat = () => {
  showShare.value = false
  showToast('功能待开发')
}
</script>

<style lang="stylus" scoped>
.mobile-chat {
  .message-list-box {
    padding-top 50px
    padding-bottom 10px
    overflow-x auto
    background #F5F5F5;

    .van-cell {
      background none
      font-family: 'Microsoft YaHei', '微软雅黑', Arial, sans-serif;
    }
  }

  .chat-box {
    .icon-box {
      .van-icon {
        font-size 24px
        margin-left 10px;
      }
    }
  }
}

.van-theme-dark {
  .mobile-chat {
    .message-list-box {
      background #232425;
    }
  }
}
</style>

<style lang="stylus">
.mobile-chat {
  .van-nav-bar__title {
    .van-dropdown-menu__title {
      margin-right 10px
    }

    .van-cell__title {
      text-align left
    }
  }

  .van-nav-bar__right {
    .van-icon {
      font-size 20px
    }
  }
}
</style>