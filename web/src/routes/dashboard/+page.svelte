<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, formatDate, type Match, type Profile } from '#lib/api';
  import {
    activitySummary,
    buildDashboardMatches,
    buildKpis,
    chartGeometry,
    filterMatches,
    monthlyActivity,
    playWindows,
    type DashboardMatch,
    type Kpi,
    type MonthActivity,
    type PlayWindow
  } from '#lib/dashboard';

  let rawMatches: Match[] = [];
  let profile: Profile | null = null;
  let loading = true;
  let error = '';
  let query = '';
  let resultFilter: 'all' | 'W' | 'L' = 'all';
  let period: '12' | '6' = '12';
  let chart = chartGeometry([0]);
  let monthly: MonthActivity[] = [];
  let summary = '';
  let windows: { totalHours: number; windows: PlayWindow[] } = { totalHours: 0, windows: [] };

  const segments = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];

  let matches: DashboardMatch[] = [];
  let kpis: Kpi[] = [];
  let filtered: DashboardMatch[] = [];

  $: matches = buildDashboardMatches(rawMatches);
  $: kpis = buildKpis(matches);
  $: monthly = monthlyActivity(matches).slice(period === '6' ? -6 : 0);
  $: chart = chartGeometry(monthly.map((month) => month.count));
  $: summary = activitySummary(monthly);
  $: windows = playWindows(profile?.availability ?? []);
  $: filtered = filterMatches(matches, query, resultFilter);

  onMount(async () => {
    const [matchesResult, profileResult] = await Promise.allSettled([
      apiJSON<{ matches: Match[] }>('/api/v1/matches'),
      apiJSON<Profile>('/api/v1/profile')
    ]);
    if (matchesResult.status === 'fulfilled') {
      rawMatches = matchesResult.value.matches;
    } else {
      error = matchesResult.reason instanceof Error ? matchesResult.reason.message : 'Matches are unavailable.';
    }
    if (profileResult.status === 'fulfilled') {
      profile = profileResult.value;
    }
    loading = false;
  });

  function exportCsv() {
    const rows = [
      ['ID', 'Opponent', 'Score', 'Venue', 'Result', 'Last updated'],
      ...filtered.map((match) => [
        match.id,
        match.opponent,
        match.score,
        match.venue,
        match.result === 'W' ? 'Win' : 'Loss',
        formatDate(match.playedAt)
      ])
    ];
    const csv = rows
      .map((row) => row.map((cell) => `"${String(cell).replaceAll('"', '""')}"`).join(','))
      .join('\n');
    const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'tennis-connect-matches.csv';
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
</script>

<svelte:head>
  <title>Overview · Tennis Connect</title>
</svelte:head>

<div class="dashboard page">
  <aside class="dash-sidebar" aria-label="Dashboard navigation">
    <a class="sidebar-brand" href="/">
      <span class="tennis-ball" aria-hidden="true"></span>
      Tennis Connect
    </a>

    <label class="sidebar-search">
      <span class="visually-hidden">Search matches</span>
      <i class="bi bi-search" aria-hidden="true"></i>
      <input type="search" placeholder="Search matches" bind:value={query} />
    </label>

    <nav class="sidebar-nav" aria-label="Dashboard">
      <a class="active" href="/dashboard" aria-current="page"><i class="bi bi-grid-1x2" aria-hidden="true"></i> Overview</a>
      <a href="/matches"><i class="bi bi-people" aria-hidden="true"></i> Matches</a>
      <a href="/discover"><i class="bi bi-compass" aria-hidden="true"></i> Discover</a>
      <a href="/profile"><i class="bi bi-person" aria-hidden="true"></i> You</a>
      <a href="/settings"><i class="bi bi-sliders" aria-hidden="true"></i> Settings</a>
    </nav>

    <p class="sidebar-foot muted">Scores, venues and results are sample data. Matches and play windows are yours.</p>
  </aside>

  <div class="dash-main">
    <header class="dash-topbar">
      <div>
        <p class="score mb-1">Dashboard</p>
        <h1>Overview</h1>
      </div>
      <a class="btn btn-primary btn-sm" href="/discover"><i class="bi bi-search" aria-hidden="true"></i> Find a match</a>
    </header>

    <p class="visually-hidden" role="status">{loading ? 'Loading dashboard' : ''}</p>

    {#if loading}
      <p role="status">Loading your dashboard…</p>
    {:else}
      {#if error}
        <p class="alert-box" role="alert">{error}</p>
      {/if}

      <p class="sample-note" role="note">
        <span class="chip sample-chip">Sample</span>
        Scores, venues, results and court hours are illustrative. Opponents, dates, matches and play
        windows come from your account.
      </p>

      <section class="kpi-strip" aria-label="Key metrics">
        {#each kpis as kpi}
          <article class="kpi">
            <h2 class="kpi-label">{kpi.label}</h2>
            <p class="kpi-value">{kpi.value}{#if kpi.unit}<span class="kpi-unit">{kpi.unit}</span>{/if}</p>
            <p class="kpi-delta" class:up={kpi.direction === 'up'} class:down={kpi.direction === 'down'}>
              {#if kpi.direction === 'up'}
                <i class="bi bi-arrow-up-right" aria-hidden="true"></i>
              {:else if kpi.direction === 'down'}
                <i class="bi bi-arrow-down-right" aria-hidden="true"></i>
              {/if}
              <span>{kpi.delta}</span>
            </p>
          </article>
        {/each}
      </section>

      <div class="dash-grid">
        <section class="dash-card chart-card" aria-labelledby="activity-title">
          <div class="card-head">
            <div>
              <p class="card-kicker">Activity</p>
              <h2 id="activity-title">Matches over time</h2>
              <p class="big-number">{monthly.reduce((total, month) => total + month.count, 0)}</p>
              <p class="muted mb-0">matches in the selected period</p>
            </div>
            <label class="period-select">
              <span class="visually-hidden">Chart period</span>
              <select bind:value={period}>
                <option value="12">Yearly</option>
                <option value="6">Last 6 months</option>
              </select>
            </label>
          </div>

          <figure class="chart-figure">
            <svg
              class="area-chart"
              viewBox={`0 0 ${chart.width} ${chart.height}`}
              role="img"
              aria-labelledby="chart-summary"
            >
              <desc id="chart-summary">{summary}</desc>
              {#each chart.ticks as tick}
                <line
                  class="grid-line"
                  x1={chart.left}
                  x2={chart.width - chart.right}
                  y1={tick.y}
                  y2={tick.y}
                />
                <text class="axis-label" x={chart.left - 6} y={tick.y + 3} text-anchor="end">{tick.label}</text>
              {/each}
              <path class="chart-area" d={chart.area} />
              <path class="chart-line" d={chart.line} />
              {#each chart.points as point}
                <circle class="chart-dot" cx={point.x} cy={point.y} r="3" />
              {/each}
              {#each monthly as month, index}
                <text
                  class="axis-label"
                  x={chart.points[index]?.x ?? 0}
                  y={chart.height - 8}
                  text-anchor="middle">{month.label}</text>
              {/each}
            </svg>
            <figcaption class="chart-summary muted">{summary}</figcaption>
          </figure>
        </section>

        <section class="dash-card side-card" aria-labelledby="windows-title">
          <div class="card-head">
            <div>
              <p class="card-kicker">Availability</p>
              <h2 id="windows-title">Weekly play windows</h2>
            </div>
            <a href="/profile/play-times">Edit</a>
          </div>

          <p class="big-number">{windows.totalHours}<span class="unit">h</span></p>
          <p class="muted">available each week</p>

          {#if windows.windows.length}
            <ul class="progress-list">
              {#each windows.windows as slot}
                <li>
                  <div class="progress-row">
                    <span>{slot.label}</span>
                    <span class="progress-value">{slot.hours}h</span>
                  </div>
                  <div class="segmented" role="img" aria-label={`${slot.label}: ${slot.hours} hours`}>
                    {#each segments as segment}
                      <span class="segment" class:on={segment < Math.round(slot.percent / 10)}></span>
                    {/each}
                  </div>
                </li>
              {/each}
            </ul>
          {:else}
            <p class="muted mb-0">No play windows saved yet.</p>
          {/if}
        </section>
      </div>

      <section class="dash-card table-card" aria-labelledby="recent-title">
        <div class="card-head table-head">
          <div>
            <p class="card-kicker">History</p>
            <h2 id="recent-title">Recent matches</h2>
            <p class="muted mb-0">Scores, venues and results are sample data.</p>
          </div>
          <div class="table-controls">
            <label class="control-search">
              <span class="visually-hidden">Search recent matches</span>
              <i class="bi bi-search" aria-hidden="true"></i>
              <input type="search" placeholder="Search" bind:value={query} />
            </label>
            <label>
              <span class="visually-hidden">Filter by result</span>
              <select bind:value={resultFilter}>
                <option value="all">All results</option>
                <option value="W">Wins</option>
                <option value="L">Losses</option>
              </select>
            </label>
            <button class="btn btn-outline-secondary btn-sm" type="button" onclick={exportCsv}>
              <i class="bi bi-download" aria-hidden="true"></i> Export
            </button>
          </div>
        </div>

        <div class="table-scroll" role="region" aria-labelledby="recent-title">
          <table>
            <caption class="visually-hidden">Recent matches. Scores, venues and results are sample data.</caption>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Opponent</th>
                <th scope="col">Score</th>
                <th scope="col">Venue</th>
                <th scope="col">Result</th>
                <th scope="col">Last updated</th>
                <th scope="col"><span class="visually-hidden">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {#each filtered as match (match.matchId)}
                <tr>
                  <td><span class="mono">{match.id}</span></td>
                  <td>{match.opponent}</td>
                  <td>{match.score}</td>
                  <td>{match.venue}</td>
                  <td>
                    <span class="badge-result" class:win={match.result === 'W'} class:loss={match.result === 'L'}>
                      <i class="bi" class:bi-trophy={match.result === 'W'} class:bi-dash-circle={match.result === 'L'} aria-hidden="true"></i>
                      {match.result === 'W' ? 'Win' : 'Loss'}
                    </span>
                  </td>
                  <td>{formatDate(match.playedAt)}</td>
                  <td>
                    <a class="row-action" href={`/matches/${match.matchId}`} aria-label={`Actions for ${match.opponent}`}>
                      <i class="bi bi-three-dots" aria-hidden="true"></i>
                    </a>
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="7" class="empty-row">
                    No matches yet. <a href="/discover">Find a player</a> to start your history.
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </section>
    {/if}
  </div>
</div>

<style>
  .dashboard {
    width: min(100% - 1.5rem, 78rem);
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    align-items: start;
    gap: 1.25rem;
    padding-block: 1.25rem 3rem;
  }

  .dash-sidebar {
    position: sticky;
    top: 5.5rem;
    display: grid;
    gap: 1rem;
    padding: 1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--surface);
    box-shadow: var(--shadow-small);
  }

  .sidebar-brand {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--ink);
    font-weight: 800;
    letter-spacing: -0.03em;
    text-decoration: none;
  }

  .sidebar-search,
  .control-search {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0 0.65rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--paper);
    color: var(--muted);
  }

  .sidebar-search input,
  .control-search input {
    min-height: 2.3rem;
    padding: 0.3rem 0;
    border: 0;
    background: transparent;
    box-shadow: none;
  }

  .sidebar-search input:focus,
  .control-search input:focus {
    box-shadow: none;
  }

  .sidebar-nav {
    display: grid;
    gap: 0.15rem;
    font-size: 0.9rem;
    font-weight: 700;
  }

  .sidebar-nav a {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.5rem 0.6rem;
    border-radius: 10px;
    color: var(--muted);
    text-decoration: none;
  }

  .sidebar-nav a:hover {
    background: var(--paper);
    color: var(--court);
  }

  .sidebar-nav a.active {
    background: var(--court);
    color: white;
  }

  .sidebar-foot {
    margin: 0;
    font-size: 0.75rem;
    line-height: 1.35;
  }

  .dash-main {
    display: grid;
    gap: 1rem;
    min-width: 0;
  }

  .dash-topbar {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .dash-topbar h1 {
    font-size: clamp(1.9rem, 4vw, 2.5rem);
    margin: 0;
  }

  .sample-note {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    margin: 0;
    color: var(--muted);
    font-size: 0.82rem;
  }

  .sample-chip {
    flex: 0 0 auto;
    background: var(--ball);
    color: #0a1714;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .kpi-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 1px;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--line);
    box-shadow: var(--shadow-small);
  }

  .kpi {
    padding: 1rem 1.1rem;
    background: var(--surface);
  }

  .kpi-label {
    margin: 0 0 0.45rem;
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.68rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .kpi-value {
    margin: 0;
    font-size: clamp(1.7rem, 3vw, 2.15rem);
    font-weight: 800;
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .kpi-unit {
    margin-left: 0.1rem;
    color: var(--muted);
    font-size: 0.7em;
  }

  .kpi-delta {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    margin: 0.55rem 0 0;
    color: var(--muted);
    font-size: 0.75rem;
  }

  .kpi-delta.up {
    color: #1c7c54;
  }

  .kpi-delta.down {
    color: var(--clay-deep);
  }

  .dash-grid {
    display: grid;
    grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr);
    gap: 1rem;
    align-items: stretch;
  }

  .dash-card {
    min-width: 0;
    padding: 1.1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--surface);
    box-shadow: var(--shadow-small);
  }

  .card-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .card-head h2 {
    margin: 0 0 0.2rem;
    font-size: 1rem;
  }

  .card-kicker {
    margin: 0 0 0.2rem;
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.66rem;
    letter-spacing: 0.07em;
    text-transform: uppercase;
  }

  .big-number {
    margin: 0.3rem 0 0;
    font-size: clamp(1.8rem, 3.4vw, 2.4rem);
    font-weight: 800;
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .unit {
    margin-left: 0.15rem;
    color: var(--muted);
    font-size: 0.55em;
  }

  .period-select {
    flex: 0 0 auto;
  }

  .period-select select {
    width: auto;
    min-height: 2.2rem;
    padding: 0.25rem 0.6rem;
    font-size: 0.8rem;
  }

  .chart-figure {
    margin: 0;
  }

  .area-chart {
    display: block;
    width: 100%;
    height: auto;
    overflow: visible;
  }

  .chart-area {
    fill: var(--dash-chart-fill);
  }

  .chart-line {
    fill: none;
    stroke: var(--dash-chart-line);
    stroke-width: 2;
  }

  .chart-dot {
    fill: var(--surface);
    stroke: var(--dash-chart-line);
    stroke-width: 2;
  }

  .grid-line {
    stroke: var(--dash-grid);
    stroke-width: 1;
  }

  .axis-label {
    fill: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 10px;
  }

  .chart-summary {
    margin: 0.4rem 0 0;
    font-size: 0.78rem;
  }

  .progress-list {
    display: grid;
    gap: 0.8rem;
    margin: 1rem 0 0;
    padding: 0;
    list-style: none;
  }

  .progress-row {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.35rem;
    font-size: 0.85rem;
  }

  .progress-value {
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.75rem;
  }

  .segmented {
    display: flex;
    gap: 2px;
    height: 6px;
  }

  .segment {
    flex: 1;
    border-radius: 999px;
    background: var(--line);
  }

  .segment.on {
    background: var(--dash-progress);
  }

  .table-head {
    align-items: flex-start;
  }

  .table-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
  }

  .table-controls .control-search {
    width: 10rem;
  }

  .table-controls select {
    width: auto;
    min-height: 2.3rem;
    padding: 0.25rem 0.6rem;
    font-size: 0.8rem;
  }

  .table-scroll {
    overflow-x: auto;
    border-top: 1px solid var(--line);
  }

  table {
    width: 100%;
    min-width: 46rem;
    border-collapse: collapse;
    font-size: 0.88rem;
  }

  th {
    padding: 0.65rem 0.5rem;
    border-bottom: 1px solid var(--line);
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.66rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-align: left;
    text-transform: uppercase;
    white-space: nowrap;
  }

  td {
    padding: 0.7rem 0.5rem;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
  }

  tbody tr:last-child td {
    border-bottom: 0;
  }

  .mono {
    color: var(--muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.76rem;
  }

  .badge-result {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.2rem 0.55rem;
    border-radius: 999px;
    font-size: 0.75rem;
    font-weight: 700;
    white-space: nowrap;
  }

  .badge-result.win {
    background: #e2f2e8;
    color: #17663f;
  }

  .badge-result.loss {
    background: #f7e3dd;
    color: var(--clay-deep);
  }

  .row-action {
    display: inline-grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    border: 1px solid transparent;
    border-radius: 999px;
    color: var(--muted);
    text-decoration: none;
  }

  .row-action:hover {
    border-color: var(--line);
    background: var(--paper);
    color: var(--court);
  }

  .empty-row {
    padding-block: 1.5rem;
    color: var(--muted);
    text-align: center;
  }

  @media (max-width: 991px) {
    .dashboard {
      grid-template-columns: minmax(0, 1fr);
    }

    .dash-sidebar {
      display: none;
    }
  }

  @media (max-width: 900px) {
    .dash-grid {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  @media (max-width: 767px) {
    .kpi-strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .table-head {
      flex-direction: column;
    }

    .table-controls {
      width: 100%;
    }

    .table-controls .control-search {
      flex: 1 1 10rem;
    }
  }

  @media (max-width: 420px) {
    .kpi-strip {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  :global([data-bs-theme='dark']) .kpi-delta.up {
    color: #6fd3a7;
  }

  :global([data-bs-theme='dark']) .badge-result.win {
    background: #123326;
    color: #8fe3b8;
  }

  :global([data-bs-theme='dark']) .badge-result.loss {
    background: #3a211b;
    color: #f0b9a8;
  }
</style>
