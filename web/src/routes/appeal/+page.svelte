<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, formatDate } from '#lib/api';

  type AppealStatus = {
    deadline: string;
    submitted: boolean;
    open: boolean;
    submitted_at?: string;
    body?: string;
  };

  let status: AppealStatus | null = null;
  let body = '';
  let error = '';
  let loading = true;
  let submitting = false;

  onMount(async () => {
    try {
      status = await apiJSON<AppealStatus>('/api/v1/account/appeal');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'No suspension is attached to this account.';
    } finally {
      loading = false;
    }
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    submitting = true;
    try {
      await apiJSON('/api/v1/account/appeal', {
        method: 'POST',
        body: JSON.stringify({ body })
      });
      status = status ? { ...status, submitted: true, body } : null;
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not submit that appeal.';
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Appeal a suspension · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if loading}
    <p role="status">Checking your account…</p>
  {:else if !status}
    <h1>No suspension</h1>
    <p class="alert-box" role="alert">{error}</p>
    <a class="btn btn-primary" href="/">Back</a>
  {:else}
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Account suspended</p>
    <h1>Your account is suspended</h1>
    <p>
      A reviewer found a serious breach of the safety rules. You can send one appeal before
      {formatDate(status.deadline)}. The reviewer sees your appeal but not your e-mail, and you will
      not see who reported you.
    </p>
    {#if status.submitted}
      <div class="surface stack">
        <h2>Appeal submitted</h2>
        <p>Sent {status.submitted_at ? formatDate(status.submitted_at) : ''}. You cannot submit a second appeal.</p>
        <p class="muted">{status.body}</p>
      </div>
    {:else if !status.open}
      <p class="alert-box" role="alert">The 14-day appeal window has closed.</p>
    {:else}
      {#if error}
        <p class="alert-box" role="alert">{error}</p>
      {/if}
      <form class="surface stack" onsubmit={submit}>
        <label>
          Why should the decision be reviewed?
          <textarea class="form-control" required maxlength="2000" bind:value={body}></textarea>
          <small>{body.length}/2000. Do not include your own or anyone else's contact details.</small>
        </label>
        <button class="btn btn-primary" type="submit" disabled={submitting}>{submitting ? 'Sending…' : 'Send appeal'}</button>
      </form>
    {/if}
  {/if}
</section>
