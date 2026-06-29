<script lang="ts">
  import PermissionsPanel from "./PermissionsPanel.svelte";
  import { OpenURL } from "../../wailsjs/go/main/App.js";
  export let onClose: () => void;

  function handleKey(e: KeyboardEvent) {
    if (e.key === "Escape") onClose();
  }

  function openPrivacyRoot() {
    OpenURL("x-apple.systempreferences:com.apple.preference.security?Privacy").catch(() => {});
  }
</script>

<svelte:window on:keydown={handleKey} />

<div class="overlay">
  <div class="modal">
    <div class="header">
      <span class="title">CSM 권한 안내</span>
      <button class="close" on:click={onClose}>✕</button>
    </div>

    <div class="intro">
      <p>macOS 권한 팝업이 잦은 이유:</p>
      <ul>
        <li>csm.app이 <strong>코드 서명되지 않아</strong> 매 업데이트마다 다른 앱처럼 보임 → TCC가 모든 권한을 다시 묻음.</li>
        <li>v0.9.42부터 안정적인 서명 적용. 이후 업데이트는 권한 기억함.</li>
        <li>지금은 한번 다 허용하면 다음 업데이트부터 조용해짐.</li>
      </ul>
      <button class="open-settings" on:click={openPrivacyRoot}>
        시스템 설정 → 개인정보 보호 열기
      </button>
    </div>

    <div class="body">
      <PermissionsPanel />
    </div>

    <div class="footer">
      <button class="confirm" on:click={onClose}>확인 (Esc)</button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.75);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 998;
  }

  .modal {
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 4px;
    width: 560px;
    max-width: 92vw;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.8);
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
  }

  .title {
    color: var(--fg);
    font-size: var(--ui-fs);
    letter-spacing: 1px;
  }

  .close {
    color: var(--fg-mute);
    padding: 2px 8px;
    border: 1px solid var(--border);
    border-radius: 2px;
  }

  .close:hover {
    color: var(--accent-action);
    border-color: var(--accent-action);
  }

  .intro {
    padding: 10px 16px;
    color: var(--fg-dim);
    font-size: var(--ui-fs-sm);
    line-height: 1.5;
    border-bottom: 1px solid var(--border);
  }
  .intro p { margin: 0 0 6px 0; }
  .intro ul { margin: 0 0 10px 18px; padding: 0; }
  .intro li { margin: 2px 0; }
  .intro strong { color: var(--accent-action); }
  .open-settings {
    background: var(--accent-folder, #4a9eff);
    color: #000;
    border: 0;
    padding: 6px 12px;
    border-radius: 3px;
    font-size: var(--ui-fs-sm);
    font-weight: 600;
    cursor: pointer;
  }
  .open-settings:hover { box-shadow: 0 0 8px rgba(74, 158, 255, 0.5); }

  .body {
    padding: 12px 16px;
    overflow-y: auto;
    flex: 1;
  }

  .footer {
    padding: 10px 16px;
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: flex-end;
  }

  .confirm {
    padding: 6px 14px;
    color: var(--fg);
    border: 1px solid var(--fg-mute);
    border-radius: 2px;
  }

  .confirm:hover {
    border-color: var(--fg);
    box-shadow: 0 0 6px var(--fg-mute);
  }
</style>
