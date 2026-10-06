<template>
  <div class="view-account">
    <div class="view-account-background">
      <div class="line line-1"></div>
      <div class="line line-2"></div>
      <div class="line line-3"></div>
      <div class="square square-1"></div>
      <div class="square square-2"></div>
      <div class="triangle"></div>
      <div class="wave wave-1"></div>
      <div class="wave wave-2"></div>
      <div class="wave wave-3"></div>
    </div>

    <div class="view-account-container animate__animated animate__fadeInDown">
      <div class="view-account-top">
        <div class="view-account-top-logo">
          <img :src="websiteConfig.loginImage" alt="" />
        </div>
        <div class="view-account-top-desc">{{ websiteConfig.loginDesc }}</div>
      </div>

      <div class="view-account-form">
        <n-form
          ref="formRef"
          label-placement="left"
          size="large"
          :model="formInline"
          :rules="rules"
        >
          <n-form-item path="username">
            <n-input
              v-model:value="formInline.username"
              placeholder="用户名"
              @keyup.enter="handleSubmit"
            >
              <template #prefix>
                <n-icon size="18" color="#808695"><PersonOutline /></n-icon>
              </template>
            </n-input>
          </n-form-item>

          <n-form-item path="password">
            <n-input
              v-model:value="formInline.password"
              type="password"
              show-password-on="click"
              placeholder="密码"
              @keyup.enter="handleSubmit"
            >
              <template #prefix>
                <n-icon size="18" color="#808695"><LockClosedOutline /></n-icon>
              </template>
            </n-input>
          </n-form-item>

          <n-form-item>
            <n-button
              type="primary"
              @click="handleSubmit"
              size="large"
              :loading="loading"
              block
            >
              登录
            </n-button>
          </n-form-item>
        </n-form>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { reactive, ref, onMounted } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { useUserStore } from '@/store/modules/user';
  import { useMessage } from 'naive-ui';
  import { ResultEnum } from '@/enums/httpEnum';
  import { PersonOutline, LockClosedOutline } from '@vicons/ionicons5';
  import { PageEnum } from '@/enums/pageEnum';
  import { websiteConfig } from '@/config/website.config';

  onMounted(() => {
    // 自动聚焦用户名输入框
    setTimeout(() => {
      const el = document.querySelector('input[placeholder="用户名"]') as HTMLElement;
      el?.focus();
    }, 300);
  });

  const formRef = ref();
  const message = useMessage();
  const loading = ref(false);
  const userStore = useUserStore();
  const router = useRouter();
  const route = useRoute();

  const formInline = reactive({
    username: '',
    password: '',
  });

  const rules = {
    username: { required: true, message: '请输入用户名', trigger: 'blur' },
    password: { required: true, message: '请输入密码', trigger: 'blur' },
  };

  /**
   * 提交登录表单。
   * 登录成功后 Cookie 由后端 Set-Cookie 写入，路由守卫自动调用 getInfo 加载菜单。
   */
  const handleSubmit = (e?: Event) => {
    e?.preventDefault();
    formRef.value.validate(async (errors: boolean) => {
      if (errors) {
        message.warning('请输入用户名和密码');
        return;
      }

      loading.value = true;
      try {
        const { username, password } = formInline;
        const res = await userStore.login({ username, password });
        const { code } = res;

        if (code === ResultEnum.SUCCESS) {
          message.success('登录成功');
          const toPath = decodeURIComponent(
            (route.query?.redirect as string) || PageEnum.BASE_HOME
          );
          router.replace(toPath);
        } else {
          message.error('用户名或密码错误');
        }
      } catch (e: unknown) {
        const errMsg = e instanceof Error ? e.message : '登录失败，请检查网络连接';
        message.error(errMsg);
      } finally {
        loading.value = false;
      }
    });
  };
</script>

<style lang="less" scoped>
  .view-account {
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: auto;
    background-color: #f0f2f5;
    background: linear-gradient(140deg, #e8f1fa, #c2d9ec, #a1c3e0, #80aed3);
    position: relative;

    &-container {
      padding: 40px;
      max-width: 420px;
      width: 100%;
      margin: 0 auto;
      background: rgba(255, 255, 255, 0.95);
      border-radius: 8px;
      box-shadow: 0 15px 35px rgba(0, 0, 0, 0.1);
      margin: auto;
      position: relative;
      z-index: 1;
      border: 1px solid rgba(255, 255, 255, 0.18);
    }

    &-top {
      text-align: center;
      margin-bottom: 24px;

      &-logo img {
        height: 48px;
      }

      &-desc {
        font-size: 14px;
        color: #909399;
        margin-top: 8px;
      }
    }
  }

  .view-account-background {
    position: absolute;
    width: 100%;
    height: 100%;
    top: 0;
    left: 0;
    overflow: hidden;
    pointer-events: none;
    z-index: 0;

    .line {
      position: absolute;
      background: linear-gradient(90deg, rgba(45, 140, 240, 0.15), rgba(0, 129, 255, 0.05));
      &-1 { width: 300px; height: 2px; top: 15%; right: 5%; transform: rotate(-30deg); }
      &-2 { width: 200px; height: 2px; bottom: 20%; left: 10%; transform: rotate(45deg); }
      &-3 { width: 150px; height: 2px; top: 40%; left: 5%; transform: rotate(-15deg); }
    }

    .square {
      position: absolute;
      &-1 {
        width: 80px; height: 80px; top: 10%; left: 15%;
        background: linear-gradient(45deg, rgba(45, 140, 240, 0.1), rgba(0, 129, 255, 0.03));
        transform: rotate(30deg);
      }
      &-2 {
        width: 60px; height: 60px; bottom: 15%; right: 10%;
        border: 2px solid rgba(45, 140, 240, 0.08);
      }
    }

    .triangle {
      position: absolute; bottom: 30%; right: 20%;
      width: 0; height: 0;
      border-left: 50px solid transparent;
      border-right: 50px solid transparent;
      border-bottom: 80px solid rgba(45, 140, 240, 0.06);
    }

    .wave {
      position: absolute; bottom: 0; left: 0; width: 100%;
      opacity: 0.2;
      &-1 { height: 120px; background: url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxNDQwIDMyMCI+PHBhdGggZmlsbD0icmdiYSg0NSwgMTQwLCAyNDAsIDAuMikiIGQ9Ik0wLDMyMEwwLDE2MEM0MCwxNjAsODAsMjQwLDEyMCwyNDBDMTYwLDI0MCwyMDAsMTYwLDI0MCwxNjBDMjgwLDE2MCwzMjAsMjQwLDM2MCwyNDBDNDAwLDI0MCw0NDAsMTYwLDQ4MCwxNjBDNTIwLDE2MCw1NjAsMjQwLDYwMCwyNDBDNjQwLDI0MCw2ODAsMTYwLDcyMCwxNjBDNzYwLDE2MCw4MDAsMjQwLDg0MCwyNDBDODgwLDI0MCw5MjAsMTYwLDk2MCwxNjBDMTAwMCwxNjAsMTA0MCwyNDAsMTA4MCwyNDBDMTEyMCwyNDAsMTE2MCwxNjAsMTIwMCwxNjBDMTI0MCwxNjAsMTI4MCwyNDAsMTMyMCwyNDBDMTM2MCwyNDAsMTQwMCwxNjAsMTQ0MCwxNjBMMTQ0MCwzMjBaIj48L3BhdGg+PC9zdmc+'); background-size: 100% 120px; }
      &-2 { height: 100px; transform: rotate(-1deg); background: url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxNDQwIDMyMCI+PHBhdGggZmlsbD0icmdiYSg0NSwgMTQwLCAyNDAsIDAuMTUpIiBkPSJNMCwzMjBMMCwxODBDNjAsMTgwLDEyMCwyNDAsMTgwLDI0MEMyNDAsMjQwLDMwMCwxODAsMzYwLDE4MEM0MjAsMTgwLDQ4MCwyNDAsNTQwLDI0MEM2MDAsMjQwLDY2MCwxODAsNzIwLDE4MEM3ODAsMTgwLDg0MCwyNDAsOTAwLDI0MEM5NjAsMjQwLDEwMjAsMTgwLDEwODAsMTgwQzExNDAsMTgwLDEyMDAsMjQwLDEyNjAsMjQwQzEzMjAsMjQwLDEzODAsMTgwLDE0NDAsMTgwTDE0NDAsMzIwWiI+PC9wYXRoPjwvc3ZnPg=='); background-size: 100% 100px; }
      &-3 { height: 80px; transform: rotate(-2deg); background: url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxNDQwIDMyMCI+PHBhdGggZmlsbD0icmdiYSg0NSwgMTQwLCAyNDAsIDAuMDgpIiBkPSJNMCwzMjBMMCwyMDBDMzAsMjAwLDYwLDI2MCw5MCwyNjBDMTIwLDI2MCwxNTAsMjAwLDE4MCwyMDBDMjEwLDIwMCwyNDAsMjYwLDI3MCwyNjBDMzAwLDI2MCwzMzAsMjAwLDM2MCwyMDBDMzkwLDIwMCw0MjAsMjYwLDQ1MCwyNjBDNDgwLDI2MCw1MTAsMjAwLDU0MCwyMDBDNTcwLDIwMCw2MDAsMjYwLDYzMCwyNjBDNjYwLDI2MCw2OTAsMjAwLDcyMCwyMDBDNzUwLDIwMCw3ODAsMjYwLDgxMCwyNjBDODQwLDI2MCw4NzAsMjAwLDkwMCwyMDBDOTMwLDIwMCw5NjAsMjYwLDk5MCwyNjBDMTAyMCwyNjAsMTA1MCwyMDAsMTA4MCwyMDBDMTExMCwyMDAsMTE0MCwyNjAsMTE3MCwyNjBMMTQ0MCwyMDBMMTQ0MCwzMjBaIj48L3BhdGg+PC9zdmc+'); background-size: 100% 80px; }
    }
  }
</style>
