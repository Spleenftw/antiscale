import React, { useEffect, useState } from 'react'
import { LayoutDashboard, Network, Shield, Settings, Server, Check, Key, Github, RefreshCw, Save } from 'lucide-react'

// Interfaces
interface Node {
  id: number; user_id: number; hostname: string; magic_name: string;
  public_key: string; private_ip: string; status: string;
  advertised_routes: string; approved_routes: string; accept_routes: boolean;
}
interface ACL { id: number; policy: string }
interface User { id: number; username: string; avatar_url: string; role: string }
interface AuthKey { id: number; key: string; auto_approve: boolean; is_reusable: boolean; is_used: boolean; created_at: string; }

function App() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [acl, setAcl] = useState<ACL | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [authKeys, setAuthKeys] = useState<AuthKey[]>([])
  
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('dashboard')
  const [editingAcl, setEditingAcl] = useState('')
  const [autoApproveNext, setAutoApproveNext] = useState(true)

  // Polling mechanism
  const fetchState = async () => {
    try {
      // 1. Check Session
      const userRes = await fetch('http://localhost:8080/api/auth/me', {credentials: 'include'})
      if (!userRes.ok) {
        setUser(null)
        setLoading(false)
        return
      }
      const userData = await userRes.json()
      setUser(userData)

      // 2. Fetch Dashboard Data
      const [nodesRes, aclRes, keysRes] = await Promise.all([
        fetch('http://localhost:8080/api/nodes', {credentials: 'include'}),
        fetch('http://localhost:8080/api/acl', {credentials: 'include'}),
        fetch('http://localhost:8080/api/auth_keys', {credentials: 'include'})
      ])
      
      if (nodesRes.ok) setNodes(await nodesRes.json())
      if (keysRes.ok) setAuthKeys(await keysRes.json())
      if (aclRes.ok) {
        const aclData = await aclRes.json()
        setAcl(aclData)
        if (!editingAcl) setEditingAcl(aclData.policy)
      }
    } catch (err) {
      console.error("Fetch error", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchState()
    const interval = setInterval(fetchState, 5000)
    return () => clearInterval(interval)
  }, [])

  // API Mutators
  const performAction = async (url: string, method: string, body?: any) => {
    await fetch(url, {
      method, credentials: 'include',
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : undefined
    })
    fetchState() // Refresh right away
  }

  const approveNode = (id: number) => performAction(`http://localhost:8080/api/nodes/${id}/approve`, 'PUT')
  const approveRoute = (id: number, route: string) => performAction(`http://localhost:8080/api/nodes/${id}/routes`, 'PUT', { approved_routes: route })
  const generateKey = () => performAction('http://localhost:8080/api/auth_keys', 'POST', { auto_approve: autoApproveNext })
  const saveAcl = async () => {
    try {
      await performAction('http://localhost:8080/api/acl', 'PUT', { policy: editingAcl })
      alert("ACL Saved Successfully")
    } catch (err) { alert("Failed to save ACL. Ensure JSON is valid.") }
  }

  const loginWithGitHub = () => {
    window.location.href = "http://localhost:8080/api/auth/github";
  }

  // --- RENDERERS ---

  if (loading) {
    return <div style={{height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>Loading mesh state...</div>
  }

  if (!user) {
    return (
      <div style={{height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg-dark)'}}>
        <div style={{background: 'var(--bg-card)', padding: '4rem 3rem', borderRadius: 'var(--radius-lg)', border: '1px solid var(--border-color)', textAlign: 'center', minWidth: '420px', backdropFilter: 'blur(20px)', boxShadow: '0 20px 40px rgba(0,0,0,0.4)', animation: 'fadeIn 0.8s ease'}}>
          <div className="logo-icon" style={{margin: '0 auto 2rem', width: '56px', height: '56px'}}></div>
          <h2 style={{marginBottom: '0.75rem', fontSize: '1.8rem'}}>Welcome to Antiscale</h2>
          <p style={{color: 'var(--text-secondary)', marginBottom: '3rem', fontSize: '0.95rem'}}>Authenticate to manage your personalized zero-trust mesh.</p>
          <button className="btn" onClick={loginWithGitHub} style={{width: '100%', justifyContent: 'center', padding: '14px', fontSize: '1rem', background: 'linear-gradient(135deg, #1f2937, #111827)'}}>
            <Github size={22} style={{marginRight: '8px'}} /> Continue with GitHub
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="dashboard-container">
      {/* SIDEBAR */}
      <aside className="sidebar">
        <div className="logo-area">
          <div className="logo-icon"></div>
          Antiscale
        </div>
        <nav className="nav-menu">
          <div className={`nav-item ${activeTab === 'dashboard' ? 'active' : ''}`} onClick={() => setActiveTab('dashboard')}>
            <LayoutDashboard size={20} /> Dashboard
          </div>
          <div className={`nav-item ${activeTab === 'keys' ? 'active' : ''}`} onClick={() => setActiveTab('keys')}>
            <Key size={20} /> Pre-Auth Keys
          </div>
          <div className={`nav-item ${activeTab === 'acl' ? 'active' : ''}`} onClick={() => setActiveTab('acl')}>
            <Shield size={20} /> Access Config
          </div>
        </nav>
        <div style={{marginTop: 'auto', display: 'flex', alignItems: 'center', gap: '12px', padding: '1rem', borderTop: '1px solid var(--border-color)'}}>
          <img src={user.avatar_url} alt="Avatar" style={{width: '32px', height: '32px', borderRadius: '50%'}} />
          <div style={{fontSize: '0.85rem'}}>
            <div style={{fontWeight: '600'}}>{user.username}</div>
            <div style={{color: 'var(--text-secondary)'}}>Admin</div>
          </div>
        </div>
      </aside>

      {/* MAIN CONTENT DIV */}
      <main className="main-content">

        {/* --- DASHBOARD TAB --- */}
        {activeTab === 'dashboard' && (
          <>
            <div className="header">
              <div><h1>Network Overview</h1><div className="subtitle">Manage and monitor your decentralized mesh</div></div>
            </div>

            <div className="stats-row">
              <div className="stat-card">
                <div className="stat-label">Total Devices</div>
                <div className="stat-value">{nodes.length}</div>
              </div>
              <div className="stat-card">
                <div className="stat-label">Active Connections</div>
                <div className="stat-value" style={{color: 'var(--status-online)'}}>{nodes.filter(n => n.status === 'approved').length}</div>
              </div>
            </div>

            <div className="panel">
              <div className="panel-header"><div className="panel-title">Registered Devices</div></div>
              <div className="table-wrapper">
                <table>
                  <thead>
                    <tr><th>Device</th><th>Antiscale IP</th><th>Subnets / Exit Nodes</th><th>Options</th><th>Status</th><th>Actions</th></tr>
                  </thead>
                  <tbody>
                    {nodes.length === 0 ? (
                      <tr><td colSpan={6} className="empty-state">No devices found. Run a client to register.</td></tr>
                    ) : nodes.map(node => (
                      <tr key={node.id}>
                        <td>
                          <div className="node-name">
                            <Server size={18} color="var(--accent-primary)" />
                            <div>
                              <div style={{fontWeight: 600}}>{node.magic_name || node.hostname}</div>
                              <div style={{fontSize: '0.75rem', color: 'var(--text-secondary)'}}>{node.hostname}</div>
                            </div>
                          </div>
                        </td>
                        <td><span className="node-ip">{node.private_ip}</span></td>
                        <td>
                          {node.advertised_routes ? (
                            <div style={{fontSize: '0.85rem'}}>
                              <div>Advertises: <code style={{color: '#94a3b8'}}>{node.advertised_routes}</code></div>
                              {node.approved_routes === node.advertised_routes ? (
                                <span style={{color: 'var(--status-online)'}}>✓ Approved</span>
                              ) : ( <button className="btn btn-secondary" style={{padding: '4px 8px', fontSize: '0.7rem', marginTop: '4px'}} onClick={() => approveRoute(node.id, node.advertised_routes)}>Approve Routes</button> )}
                            </div>
                          ) : <span style={{color: 'var(--text-secondary)', fontSize: '0.85rem'}}>None</span>}
                        </td>
                        <td>
                           <span style={{fontSize: '0.8rem', color: node.accept_routes ? 'var(--status-online)' : 'var(--text-secondary)'}}>
                             Accepts Routes: {node.accept_routes ? "Yes" : "No"}
                           </span>
                        </td>
                        <td><span className={`status-badge status-${node.status}`}>{node.status}</span></td>
                        <td>
                          {node.status === 'pending' ? (
                            <button className="btn" onClick={() => approveNode(node.id)}><Check size={16} /> Approve</button>
                          ) : <button className="btn btn-secondary">Manage</button>}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </>
        )}

        {/* --- KEYS TAB --- */}
        {activeTab === 'keys' && (
          <>
            <div className="header">
              <div><h1>Pre-Auth Keys</h1><div className="subtitle">Generate keys to autonomously enroll docker clients</div></div>
            </div>
            
            <div className="panel" style={{marginBottom: '2rem'}}>
              <div className="panel-header"><div className="panel-title">Generate New Key</div></div>
              <div style={{padding: '1.5rem', display: 'flex', alignItems: 'center', gap: '1.5rem'}}>
                <label style={{display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer'}}>
                  <input type="checkbox" checked={autoApproveNext} onChange={e => setAutoApproveNext(e.target.checked)} />
                  Auto-Approve Devices
                </label>
                <button className="btn" onClick={generateKey}><RefreshCw size={16} /> Generate Auth Key</button>
              </div>
            </div>

            <div className="panel">
              <div className="panel-header"><div className="panel-title">Active Keys</div></div>
              <div className="table-wrapper">
                <table>
                  <thead><tr><th>Key</th><th>Attributes</th><th>Created At</th></tr></thead>
                  <tbody>
                    {authKeys.length === 0 ? <tr><td colSpan={3} className="empty-state">No keys generated.</td></tr> : authKeys.map(k => (
                      <tr key={k.id}>
                        <td><code style={{background: 'rgba(0,0,0,0.5)', padding: '6px 12px', borderRadius: '6px'}}>{k.key}</code></td>
                        <td>
                           <div style={{display: 'flex', gap: '10px'}}>
                             {k.auto_approve && <span className="status-badge status-approved">Auto Approve</span>}
                             {k.is_reusable && <span className="status-badge" style={{background: 'rgba(99,102,241,0.2)', color: '#818cf8'}}>Reusable</span>}
                           </div>
                        </td>
                        <td style={{color: 'var(--text-secondary)'}}>{new Date(k.created_at).toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </>
        )}

        {/* --- ACL TAB --- */}
        {activeTab === 'acl' && (
          <>
            <div className="header">
              <div><h1>Access Controls</h1><div className="subtitle">Define network restrictions using HuJSON rules</div></div>
              <button className="btn" onClick={saveAcl}><Save size={16} /> Save Policy</button>
            </div>
            <div className="panel" style={{display: 'flex', flexDirection: 'column', height: 'calc(100vh - 200px)'}}>
              <textarea 
                style={{flex: 1, padding: '2rem', background: 'rgba(0,0,0,0.15)', color: 'var(--text-primary)', border: 'none', resize: 'none', fontFamily: 'SFMono-Regular, Consolas, monospace', fontSize: '14px', outline: 'none', lineHeight: '1.6'}}
                value={editingAcl} onChange={(e) => setEditingAcl(e.target.value)}
              />
            </div>
          </>
        )}
      </main>
    </div>
  )
}

export default App
