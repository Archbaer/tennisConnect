<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError, apiJSON, formatDate, type Report } from '#lib/api';

  let state: 'loading' | 'signed-out' | 'mfa' | 'queue' = 'loading';
  let email = '';
  let code = '';
  let secret = '';
  let uri = '';
  let reports: Report[] = [];
  let error = '';
  let notice = '';
  let confirmingBan: string | null = null;

  onMount(loadQueue);

  async function loadQueue() {
    state = 'loading';
    error = '';
    try {
      reports = (await apiJSON<{ reports: Report[] }>('/api/v1/reviewer/reports')).reports;
      state = 'queue';
    } catch (problem) {
      if (problem instanceof ApiError && problem.status === 403) {
        state = 'mfa';
      } else {
        state = 'signed-out';
      }
    }
  }

  async function requestLink(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    try {
      await apiJSON('/api/v1/reviewer/auth/link', {
        method: 'POST',
        body: JSON.stringify({ email })
      });
      notice = 'Check the reviewer inbox for the sign-in link.';
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Could not send that link.';
    }
  }

  async function enroll() {
    error = '';
    try {
      const result = await apiJSON<{ secret: string; otpauth_uri: string }>(
        '/api/v1/reviewer/mfa/enroll',
        { method: 'POST' }
      );
      secret = result.secret;
      uri = result.otpauth_uri;
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Could not start MFA enrollment.';
    }
  }

  async function verifyMFA(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    try {
      await apiJSON('/api/v1/reviewer/mfa/verify', {
        method: 'POST',
        body: JSON.stringify({ code })
      });
      await loadQueue();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'That code did not work.';
    }
  }

  async function decide(reportID: string, action: 'dismiss' | 'ban') {
    error = '';
    try {
      await apiJSON(`/api/v1/reviewer/reports/${reportID}/${action}`, { method: 'POST' });
      confirmingBan = null;
      await loadQueue();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'That decision did not save.';
    }
  }
</script>

<svelte:head>
  <title>Reviewer console · Tennis Connect</title>
</svelte:head>

<section class="page stack">
  <div>
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Internal</p>
    <h1>Reviewer console</h1>
  </div>

  {#if error}
    <p class="alert-box" role="alert">{error}</p>
  {/if}
  {#if notice}
    <p class="notice">{notice}</p>
  {/if}

  {#if state === 'loading'}
    <p role="status">Loading…</p>
  {:else if state === 'signed-out'}
    <form class="surface stack narrow" onsubmit={requestLink}>
      <h2>Reviewer sign in</h2>
      <label>
        Reviewer e-mail
        <input class="form-control" type="email" required bind:value={email} />
      </label>
      <button class="btn btn-primary" type="submit">Send reviewer link</button>
    </form>
  {:else if state === 'mfa'}
    <div class="surface stack narrow">
      <h2>Second factor</h2>
      <p class="muted">Enter the six-digit code from the authenticator. If this is the first sign-in, enroll first.</p>
      {#if secret}
        <p class="notice">Add this secret to the authenticator, then enter a code.</p>
        <code class="secret">{secret}</code>
        <small>{uri}</small>
      {/if}
      <form class="stack" onsubmit={verifyMFA}>
        <label>
          Code
          <input class="form-control" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" bind:value={code} />
        </label>
        <button class="btn btn-primary" type="submit">Verify code</button>
      </form>
      <button class="btn btn-outline-secondary" onclick={enroll}>Enroll this account</button>
    </div>
  {:else}
    <div class="row spread">
      <h2>{reports.length} open reports</h2>
      <button class="btn btn-outline-secondary" onclick={loadQueue}>Refresh</button>
    </div>
    {#if !reports.length}
      <p class="notice">The queue is clear.</p>
    {:else}
      <ul class="queue">
        {#each reports as report (report.report_id)}
          <li class="card stack">
            <div class="spread">
              <h3>{report.display_name}</h3>
              <span class:alert={report.overdue} class="chip">{report.overdue ? 'Overdue' : 'Open'}</span>
            </div>
            <p class="score dotlist">
              <span>{report.category}</span>
              <span>submitted {formatDate(report.created_at)}</span>
              <span>due {formatDate(report.review_due_at)}</span>
            </p>
            {#if report.details}<p>{report.details}</p>{/if}
            <div class="row">
              <button class="btn btn-outline-secondary" onclick={() => decide(report.report_id, 'dismiss')}>Dismiss</button>
              {#if confirmingBan === report.report_id}
                <button class="btn btn-danger" onclick={() => decide(report.report_id, 'ban')}>Confirm permanent ban</button>
                <button class="btn btn-outline-secondary" onclick={() => (confirmingBan = null)}>Cancel</button>
              {:else}
                <button class="btn btn-danger" onclick={() => (confirmingBan = report.report_id)}>Ban</button>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</section>

<style>
  .queue {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.75rem;
  }

  .secret {
    overflow-wrap: anywhere;
    background: var(--paper);
    padding: 0.75rem;
    border-radius: 8px;
  }
</style>
