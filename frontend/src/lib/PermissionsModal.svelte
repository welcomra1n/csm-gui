<script lang="ts">
  import PermissionsPanel from "./PermissionsPanel.svelte";
  export let onClose: () => void;

  function handleKey(e: KeyboardEvent) {
    if (e.key === "Escape") onClose();
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
      csm-gui가 사용하는 권한들. 필수는 끌 수 없음. OS 표시는 macOS Privacy & Security 설정에서 별도 관리.
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
