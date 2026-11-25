<script lang="ts">
  import { goto } from '$app/navigation';
  import { tick } from 'svelte';
  import { apiJSON } from '#lib/api';

  let error = '';
  let recoverUntil = '';
  let deleting = false;
  let dialog: HTMLDialogElement;
  let cancelButton: HTMLButtonElement;

  async function openConfirm() {
    if (!dialog.open) dialog.showModal();
    await tick();
    cancelButton?.focus();
  }

  function closeConfirm() {
    dialog.close();
  }

  async function remove() {
    error = '';
    deleting = true;
    try {
      const result = await apiJSON<{ recover_until: string }>('/api/v1/account', {
        method: 'DELETE'
      });
      dialog.close();
      recoverUntil = result.recover_until;
      setTimeout(() => goto('/'), 2500);
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not delete that account.';
    } finally {
      deleting = false;
    }
  }
</script>

<svelte:head>
  <title>Delete account · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if recoverUntil}
    <p class="score">Scheduled</p>
    <h1>Your account is hidden</h1>
    <p>
      Nothing is visible to other players. Signing in again before
      {new Date(recoverUntil).toLocaleDateString('en-GB')} restores the account. After that it is
      deleted.
    </p>
    <a class="btn btn-primary" href="/">Return to the start</a>
  {:else}
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Careful</p>
    <h1>Delete account</h1>
    <p>
      Your profile, photos, matches and likes are hidden immediately. You have 30 days to recover by
      signing in again. After 30 days the account is deleted.
    </p>
    <p class="muted">
      Download your data first if you want a copy. Reports you made are used only for the safety review.
    </p>
    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}
    <div class="row">
      <button class="btn btn-danger" disabled={deleting} onclick={openConfirm}>
        {deleting ? 'Deleting…' : 'Delete my account'}
      </button>
      <a class="btn btn-outline-secondary" href="/settings">Keep my account</a>
    </div>
  {/if}
</section>

<dialog bind:this={dialog} class="delete-dialog" aria-labelledby="delete-title" aria-describedby="delete-description">
  <div class="dialog-head">
    <div>
      <p class="score mb-1">Confirm</p>
      <h2 id="delete-title" class="mb-0">Delete your account?</h2>
    </div>
  </div>
  <p id="delete-description">
    Your profile is hidden now and deleted after 30 days. Signing in before then restores it.
  </p>
  {#if error}<p class="alert-box" role="alert">{error}</p>{/if}
  <div class="dialog-actions">
    <button class="btn btn-danger" type="button" onclick={remove} disabled={deleting}>
      {deleting ? 'Deleting…' : 'Delete my account'}
    </button>
    <button class="btn btn-outline-secondary" type="button" bind:this={cancelButton} onclick={closeConfirm}>
      Cancel
    </button>
  </div>
</dialog>

<style>
  .delete-dialog {
    width: min(100% - 2rem, 30rem);
    margin: auto;
    padding: 1.5rem;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--ink);
    box-shadow: 0 28px 80px rgb(6 40 31 / 28%);
  }

  .delete-dialog::backdrop {
    background: rgb(6 40 31 / 55%);
    backdrop-filter: blur(3px);
  }

  .dialog-head {
    margin-bottom: 1rem;
  }

  .dialog-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .dialog-actions .btn {
    flex: 1 1 10rem;
  }
</style>
