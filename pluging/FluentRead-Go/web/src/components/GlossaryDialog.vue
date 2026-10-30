<!--
  GlossaryDialog.vue 是术语管理弹窗（FR-501，P4）。
  @file web/src/components/GlossaryDialog.vue
  文件职责：列出合并视图（站点/用户）、新增（可选作用域）、编辑、删除、启停；
  术语进入提示词（FR-502 由服务端保证），改动后指纹变化驱动"译文已过期"横幅。
  模块边界：数据在 stores/glossary；确认删除用 ui.confirm。
-->
<script setup lang="ts">
import { ref, watch } from 'vue';
import { useGlossaryStore } from '../stores/glossary';
import { useUiStore } from '../stores/ui';
import type { GlossaryItem } from '../api/types';

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();

const glossary = useGlossaryStore();
const ui = useUiStore();

const form = ref({ scope: 'site' as 'site' | 'user', sourceTerm: '', targetTerm: '', note: '' });
const editing = ref<GlossaryItem | null>(null);
const errMsg = ref('');

watch(
  () => props.open,
  (v) => {
    if (v) void glossary.refresh();
  },
);

async function submit() {
  errMsg.value = '';
  try {
    if (editing.value) {
      await glossary.update({
        ...editing.value,
        sourceTerm: form.value.sourceTerm,
        targetTerm: form.value.targetTerm,
        note: form.value.note,
      });
    } else {
      await glossary.add(form.value.scope, form.value.sourceTerm, form.value.targetTerm, form.value.note);
    }
    form.value = { scope: form.value.scope, sourceTerm: '', targetTerm: '', note: '' };
    editing.value = null;
  } catch (e) {
    errMsg.value = e instanceof Error ? e.message : '保存失败，请稍后重试';
  }
}

async function removeItem(item: GlossaryItem) {
  const ok = await ui.askConfirm('删除术语', `确定删除"${item.sourceTerm} → ${item.targetTerm}"吗？`, '删除');
  if (!ok) return;
  await glossary.remove(item.id);
}

function startEdit(item: GlossaryItem) {
  editing.value = item;
  form.value = { scope: item.scope, sourceTerm: item.sourceTerm, targetTerm: item.targetTerm, note: item.note ?? '' };
}
</script>

<template>
  <el-dialog
    :model-value="open"
    title="术语表"
    width="640px"
    append-to-body
    data-test="glossary-dialog"
    @update:model-value="(v: boolean) => { if (!v) emit('close'); }"
  >
    <p class="hint faint" style="margin-top:0">
      术语会写进翻译提示词：重译后指定译法会生效（站点级全站共享，用户级只属于你）。
    </p>

    <!-- 新增/编辑表单 -->
    <el-form inline size="small" class="g-form" data-test="glossary-form">
      <el-form-item>
        <el-select v-model="form.scope" style="width:96px" aria-label="作用域" :disabled="!!editing">
          <el-option value="site" label="站点级" />
          <el-option value="user" label="我的" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-input v-model="form.sourceTerm" data-test="glossary-source" placeholder="英文原文，如 recall" style="width:170px" />
      </el-form-item>
      <el-form-item>
        <el-input v-model="form.targetTerm" data-test="glossary-target" placeholder="指定译法，如 召回率" style="width:170px" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" data-test="glossary-submit" @click="submit">
          {{ editing ? '保存修改' : '添加' }}
        </el-button>
        <el-button v-if="editing" @click="editing = null">取消</el-button>
      </el-form-item>
    </el-form>
    <p v-if="errMsg" class="hint" style="color:var(--danger)" data-test="glossary-error">{{ errMsg }}</p>

    <!-- 列表 -->
    <el-table :data="glossary.items" size="small" data-test="glossary-table" max-height="320">
      <el-table-column width="80" prop="scope">
        <template #default="{ row }">
          <el-tag size="small" :type="row.scope === 'site' ? 'info' : 'success'" effect="plain">
            {{ row.scope === 'site' ? '站点' : '我的' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="sourceTerm" label="原文" min-width="140" />
      <el-table-column prop="targetTerm" label="指定译法" min-width="140" />
      <el-table-column width="180" align="right">
        <template #default="{ row }">
          <el-button size="small" text @click="startEdit(row)">编辑</el-button>
          <el-button size="small" text type="danger" :data-test="`glossary-del-${row.id}`" @click="removeItem(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="glossary.loaded && glossary.items.length === 0" description="还没有术语" :image-size="60" />
  </el-dialog>
</template>

<style scoped>
.g-form .el-form-item {
  margin-bottom: 8px;
}
</style>
