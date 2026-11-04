
import { Fragment, useEffect, useState, type FormEvent } from 'react'
import './App.css'

interface DashboardCounts {
  total_members: number
  usfw_members: number
  youth_members: number
  quakermen_members: number
}

interface LoginResponse {
  message: string
  token: string
  user: {
    id: string
    member_id: string
    email: string
  }
}

interface Member {
  id: string
  full_name: string
  email: string | null
  phone: string | null
  status: string
  congregation: string
}

interface Department {
  id: string
  name: string
  description?: string | null
  role?: string
}

interface MembersResponse {
  total: number
  members: Member[]
}

interface MemberDepartmentsResponse {
  departments: Department[]
}

const CONGREGATION_ID = '35477991-337a-49f8-9534-03c311edbab5'

const initialCounts: DashboardCounts = {
  total_members: 0,
  usfw_members: 0,
  youth_members: 0,
  quakermen_members: 0,
}

function App() {
  const [counts, setCounts] = useState<DashboardCounts>(initialCounts)
  const [loading, setLoading] = useState(true)
  const [dashboardError, setDashboardError] = useState('')

  const [page, setPage] = useState<
    'public' | 'login' | 'admin' | 'members'
  >('public')

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loginError, setLoginError] = useState('')
  const [loginLoading, setLoginLoading] = useState(false)

  const [token, setToken] = useState<string | null>(null)
  const [loggedInEmail, setLoggedInEmail] = useState('')

  const [members, setMembers] = useState<Member[]>([])
  const [departments, setDepartments] = useState<Department[]>([])
  const [selectedDepartments, setSelectedDepartments] = useState<
    Record<string, string>
  >({})
  const [memberDepartments, setMemberDepartments] = useState<
    Record<string, Department[]>
  >({})
  const [membersLoading, setMembersLoading] = useState(false)
  const [membersError, setMembersError] = useState('')
  const [memberMessage, setMemberMessage] = useState('')

  const [fullName, setFullName] = useState('')
  const [editingMemberID, setEditingMemberID] = useState<string | null>(null)
  const [editFullName, setEditFullName] = useState('')
  const [editEmail, setEditEmail] = useState('')
  const [editPhone, setEditPhone] = useState('')
  const [savingEdit, setSavingEdit] = useState(false)
  const [memberEmail, setMemberEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [savingMember, setSavingMember] = useState(false)

  useEffect(() => {
    async function loadDashboard() {
      try {
        const response = await fetch('/api/dashboard/counts')

        if (!response.ok) {
          throw new Error('Could not load dashboard')
        }

        const data: DashboardCounts = await response.json()
        setCounts(data)
      } catch {
        setDashboardError('Could not connect to the church server.')
      } finally {
        setLoading(false)
      }
    }

    loadDashboard()
  }, [])

  async function loadMembers(authToken: string): Promise<boolean> {
    setMembersLoading(true)
    setMembersError('')
    setMemberMessage('')

    try {
      const response = await fetch(
        `/api/admin/members?congregation_id=${CONGREGATION_ID}`,
        {
          headers: {
            Authorization: `Bearer ${authToken}`,
          },
        },
      )
      const departmentsResponse = await fetch('/api/departments')

      const data = await response.json()
      const departmentsData = await departmentsResponse.json()

      if (!response.ok) {
        throw new Error(data.error || 'Could not load members.')
      }
      if (!departmentsResponse.ok) {
        throw new Error(
          departmentsData.error || 'Could not load departments.',
        )
      }

      const result = data as MembersResponse
      const departmentList = departmentsData as Department[]
      const departmentEntries = await Promise.all(
        (result.members || []).map(async (member) => {
          const assignmentsResponse = await fetch(
            `/api/admin/members/departments?member_id=${encodeURIComponent(member.id)}`,
            {
              headers: {
                Authorization: `Bearer ${authToken}`,
              },
            },
          )
          const assignmentsData = await assignmentsResponse.json()

          if (!assignmentsResponse.ok) {
            throw new Error(
              assignmentsData.error ||
                `Could not load departments for ${member.full_name}.`,
            )
          }

          const assignments =
            assignmentsData as MemberDepartmentsResponse
          return [member.id, assignments.departments || []] as const
        }),
      )

      setMembers(result.members || [])
      setDepartments(departmentList)
      setMemberDepartments(Object.fromEntries(departmentEntries))
      return true
    } catch (error) {
      setMembersError(
        error instanceof Error
          ? error.message
          : 'Could not load members.',
      )
      return false
    } finally {
      setMembersLoading(false)
    }
  }

  async function handleAssignDepartment(memberID: string) {
    if (!token) {
      setMembersError('Please sign in again.')
      return
    }

    const departmentID = selectedDepartments[memberID]
    if (!departmentID) {
      setMembersError('Select a department to assign.')
      return
    }

    setMembersError('')
    setMemberMessage('')

    try {
      const response = await fetch('/api/admin/members/department', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          member_id: memberID,
          department_id: departmentID,
        }),
      })
      const data = await response.json()

      if (!response.ok) {
        throw new Error(data.error || 'Could not assign department.')
      }

      if (await loadMembers(token)) {
        setMemberMessage('Department assignment saved.')
      }
    } catch (error) {
      setMembersError(
        error instanceof Error
          ? error.message
          : 'Could not assign department.',
      )
    }
  }

  function handleOpenMembers() {
    setPage('members')

    if (token) {
      void loadMembers(token)
    }
  }

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setLoginError('')
    setLoginLoading(true)

    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          email: email.trim(),
          password,
        }),
      })

      const data = await response.json()

      if (!response.ok) {
        throw new Error(data.error || 'Login failed. Check your details.')
      }

      const loginData = data as LoginResponse

      if (!loginData.token || !loginData.user) {
        throw new Error('The server returned an invalid login response.')
      }

      setToken(loginData.token)
      setLoggedInEmail(loginData.user.email)
      setPassword('')
      setPage('admin')
    } catch (error) {
      setLoginError(
        error instanceof Error
          ? error.message
          : 'Unable to log in. Please try again.',
      )
    } finally {
      setLoginLoading(false)
    }
  }

  function handleStartEdit(member: Member) {
    setEditingMemberID(member.id)
    setEditFullName(member.full_name)
    setEditEmail(member.email || '')
    setEditPhone(member.phone || '')
    setMembersError('')
    setMemberMessage('')
  }

  async function handleSaveEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!token || !editingMemberID) {
      setMembersError('Please sign in again.')
      return
    }

    setSavingEdit(true)
    setMembersError('')
    setMemberMessage('')

    try {
      const response = await fetch('/api/admin/members/edit', {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          member_id: editingMemberID,
          full_name: editFullName.trim(),
          email: editEmail.trim() || null,
          phone: editPhone.trim() || null,
          congregation_id: CONGREGATION_ID,
        }),
      })

      const data = await response.json()

      if (!response.ok) {
        throw new Error(data.error || 'Could not update member.')
      }

      setEditingMemberID(null)
      setEditFullName('')
      setEditEmail('')
      setEditPhone('')

      if (await loadMembers(token)) {
        setMemberMessage('Member details updated successfully.')
      }
    } catch (error) {
      setMembersError(
        error instanceof Error
          ? error.message
          : 'Could not update member.',
      )
    } finally {
      setSavingEdit(false)
    }
  }

  function handleCancelEdit() {
    setEditingMemberID(null)
    setEditFullName('')
    setEditEmail('')
    setEditPhone('')
  }

  async function handleCreateMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!token) {
      setMembersError('Please sign in again.')
      return
    }

    setSavingMember(true)
    setMembersError('')
    setMemberMessage('')

    try {
      const response = await fetch('/api/admin/members', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          full_name: fullName.trim(),
          email: memberEmail.trim() || null,
          phone: phone.trim() || null,
          congregation_id: CONGREGATION_ID,
          profile_image_url: null,
        }),
      })

      const data = await response.json()

      if (!response.ok) {
        throw new Error(data.error || 'Could not register member.')
      }

      setFullName('')
      setMemberEmail('')
      setPhone('')
      if (await loadMembers(token)) {
        setMemberMessage('Member registered successfully.')
      }
    } catch (error) {
      setMembersError(
        error instanceof Error
          ? error.message
          : 'Could not register member.',
      )
    } finally {
      setSavingMember(false)
    }
  }

  function handleLogout() {
    setToken(null)
    setLoggedInEmail('')
    setEmail('')
    setPassword('')
    setMembers([])
    setDepartments([])
    setSelectedDepartments({})
    setMemberDepartments({})
    handleCancelEdit()
    setPage('public')
  }

  const cards = [
    {
      title: 'Total Members',
      value: counts.total_members,
      icon: '👥',
      description: 'All active church members',
    },
    {
      title: 'USFW',
      value: counts.usfw_members,
      icon: '🌸',
      description: 'USFW department members',
    },
    {
      title: 'Youth',
      value: counts.youth_members,
      icon: '🌱',
      description: 'Youth department members',
    },
    {
      title: 'Quakermen',
      value: counts.quakermen_members,
      icon: '🤝',
      description: 'Quakermen department members',
    },
  ]

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <div className="brand-icon">F</div>
          <div>
            <h2>Friends Church</h2>
            <span>Quakers Management System</span>
          </div>
        </div>

        <nav className="navigation">
          <button
            className="nav-link"
            onClick={() => setPage('public')}
          >
            Public Dashboard
          </button>
          <a href="#about" onClick={() => setPage('public')}>
            About Church
          </a>
        </nav>

        {token ? (
          <button className="login-button" onClick={handleLogout}>
            Logout
          </button>
        ) : (
          <button
            className="login-button"
            onClick={() => {
              setLoginError('')
              setPage('login')
            }}
          >
            Admin Login
          </button>
        )}
      </header>

      {page === 'public' && (
        <main className="main-content">
          <section className="welcome">
            <p className="eyebrow">WELCOME TO OUR CHURCH</p>
            <h1>Kimilili Town Friends Church</h1>
            <p>Growing together in faith, fellowship and service.</p>
          </section>

          <section className="dashboard" id="dashboard">
            <div className="section-heading">
              <div>
                <h2>Church Dashboard</h2>
                <p>Membership overview</p>
              </div>
              <span className="public-label">Public Information</span>
            </div>

            {loading && <p>Loading membership information...</p>}
            {dashboardError && (
              <p className="error-message">{dashboardError}</p>
            )}

            {!loading && !dashboardError && (
              <div className="cards">
                {cards.map((card) => (
                  <article className="stat-card" key={card.title}>
                    <span className="card-icon">{card.icon}</span>
                    <p className="card-title">{card.title}</p>
                    <h3>{card.value}</h3>
                    <p className="card-description">
                      {card.description}
                    </p>
                  </article>
                ))}
              </div>
            )}
          </section>

          <section className="information" id="about">
            <h2>Our Church Community</h2>
            <p>
              A community committed to worship, fellowship,
              spiritual growth and serving others.
            </p>
          </section>
        </main>
      )}

      {page === 'login' && (
        <main className="login-page">
          <form className="login-card" onSubmit={handleLogin}>
            <div className="login-symbol">F</div>

            <h1>Administrator Login</h1>
            <p className="login-subtitle">
              Sign in to manage church information.
            </p>

            <label htmlFor="email">Email address</label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="Enter your email"
              autoComplete="username"
              required
            />

            <label htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="Enter your password"
              autoComplete="current-password"
              required
            />

            {loginError && (
              <p className="error-message" role="alert">
                {loginError}
              </p>
            )}

            <button
              className="submit-login"
              type="submit"
              disabled={loginLoading}
            >
              {loginLoading ? 'Signing in...' : 'Sign In'}
            </button>

            <button
              className="back-button"
              type="button"
              onClick={() => setPage('public')}
            >
              Back to public dashboard
            </button>
          </form>
        </main>
      )}

      {page === 'admin' && token && (
        <main className="admin-page">
          <section className="admin-welcome">
            <p className="eyebrow">ADMINISTRATION</p>
            <h1>Welcome to the Admin Portal</h1>
            <p>Signed in as {loggedInEmail}</p>
          </section>

          <section className="admin-content">
            <h2>Church Management</h2>
            <p>Select a management section below.</p>

            <button
              className="submit-login"
              onClick={handleOpenMembers}
            >
              Manage Church Members
            </button>

            <p className="session-note">
              Your session token is held in memory and expires according
              to the server's 12-hour token limit.
            </p>

            <button className="back-button" onClick={handleLogout}>
              Sign Out
            </button>
          </section>
        </main>
      )}

      {page === 'members' && token && (
        <main className="admin-page">
          <section className="admin-welcome">
            <p className="eyebrow">MEMBER MANAGEMENT</p>
            <h1>Church Members</h1>
            <p>Kimilili Town Friends Church</p>
            <button
              className="back-button"
              onClick={() => setPage('admin')}
            >
              Back to Admin Portal
            </button>
          </section>

          <section className="admin-content">
            <h2>Register New Member</h2>

            <form onSubmit={handleCreateMember}>
              <label htmlFor="fullName">Full name *</label>
              <input
                id="fullName"
                value={fullName}
                onChange={(event) => setFullName(event.target.value)}
                placeholder="Enter member's full name"
                required
              />

              <label htmlFor="memberEmail">Email address</label>
              <input
                id="memberEmail"
                type="email"
                value={memberEmail}
                onChange={(event) => setMemberEmail(event.target.value)}
                placeholder="Enter email address (optional)"
              />

              <label htmlFor="memberPhone">Phone number</label>
              <input
                id="memberPhone"
                type="tel"
                value={phone}
                onChange={(event) => setPhone(event.target.value)}
                placeholder="Enter phone number (optional)"
              />

              {membersError && (
                <p className="error-message" role="alert">
                  {membersError}
                </p>
              )}

              {memberMessage && (
                <p role="status">{memberMessage}</p>
              )}

              <button
                className="submit-login"
                type="submit"
                disabled={savingMember}
              >
                {savingMember ? 'Registering...' : 'Register Member'}
              </button>
            </form>
          </section>

          <section className="admin-content">
            <h2>Registered Members ({members.length})</h2>

            <button
              className="back-button"
              onClick={() => loadMembers(token)}
              disabled={membersLoading}
            >
              {membersLoading ? 'Refreshing...' : 'Refresh Members'}
            </button>

            {membersLoading && <p>Loading members...</p>}

            {!membersLoading && members.length === 0 && (
              <p>No members found for this congregation.</p>
            )}

            {!membersLoading && members.length > 0 && (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr>
                      <th style={{ textAlign: 'left', padding: '10px' }}>Name</th>
                      <th style={{ textAlign: 'left', padding: '10px' }}>Email</th>
                      <th style={{ textAlign: 'left', padding: '10px' }}>Phone</th>
                      <th style={{ textAlign: 'left', padding: '10px' }}>Status</th>
                      <th style={{ textAlign: 'left', padding: '10px' }}>Departments</th>
                      <th style={{ textAlign: 'left', padding: '10px' }}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {members.map((member) => (
                      <Fragment key={member.id}>
                        <tr>
                          <td style={{ padding: '10px' }}>{member.full_name}</td>
                          <td style={{ padding: '10px' }}>{member.email || '-'}</td>
                          <td style={{ padding: '10px' }}>{member.phone || '-'}</td>
                          <td style={{ padding: '10px' }}>{member.status}</td>
                          <td style={{ padding: '10px' }}>
                            <div>
                              {memberDepartments[member.id]?.length
                                ? memberDepartments[member.id]
                                    .map((department) => department.name)
                                    .join(', ')
                                : 'Not assigned'}
                            </div>
                            {departments.length > 0 && (
                              <div>
                                <select
                                  aria-label={`Department for ${member.full_name}`}
                                  value={selectedDepartments[member.id] || ''}
                                  onChange={(event) =>
                                    setSelectedDepartments((current) => ({
                                      ...current,
                                      [member.id]: event.target.value,
                                    }))
                                  }
                                >
                                  <option value="">Select department</option>
                                  {departments.map((department) => (
                                    <option
                                      key={department.id}
                                      value={department.id}
                                    >
                                      {department.name}
                                    </option>
                                  ))}
                                </select>
                                <button
                                  className="back-button"
                                  type="button"
                                  onClick={() =>
                                    void handleAssignDepartment(member.id)
                                  }
                                >
                                  Assign
                                </button>
                              </div>
                            )}
                          </td>
                          <td style={{ padding: '10px' }}>
                            <button
                              className="back-button"
                              type="button"
                              onClick={() => handleStartEdit(member)}
                              disabled={savingEdit}
                            >
                              Edit
                            </button>
                          </td>
                        </tr>
                        {editingMemberID === member.id && (
                          <tr key={`${member.id}-edit`}>
                            <td colSpan={6} style={{ padding: '10px' }}>
                              <form onSubmit={handleSaveEdit}>
                                <label htmlFor={`edit-name-${member.id}`}>
                                  Full name *
                                </label>
                                <input
                                  id={`edit-name-${member.id}`}
                                  value={editFullName}
                                  onChange={(event) =>
                                    setEditFullName(event.target.value)
                                  }
                                  required
                                />
                                <label htmlFor={`edit-email-${member.id}`}>
                                  Email address
                                </label>
                                <input
                                  id={`edit-email-${member.id}`}
                                  type="email"
                                  value={editEmail}
                                  onChange={(event) =>
                                    setEditEmail(event.target.value)
                                  }
                                />
                                <label htmlFor={`edit-phone-${member.id}`}>
                                  Phone number
                                </label>
                                <input
                                  id={`edit-phone-${member.id}`}
                                  type="tel"
                                  value={editPhone}
                                  onChange={(event) =>
                                    setEditPhone(event.target.value)
                                  }
                                />
                                <button
                                  className="submit-login"
                                  type="submit"
                                  disabled={savingEdit}
                                >
                                  {savingEdit ? 'Saving...' : 'Save Changes'}
                                </button>
                                <button
                                  className="back-button"
                                  type="button"
                                  onClick={handleCancelEdit}
                                  disabled={savingEdit}
                                >
                                  Cancel
                                </button>
                              </form>
                            </td>
                          </tr>
                        )}
                      </Fragment>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </main>
      )}

      <footer>
        <p>
          © {new Date().getFullYear()} Kimilili Town Friends Church
        </p>
        <p>Friends Church Quakers Management System</p>
      </footer>
    </div>
  )
}

export default App
