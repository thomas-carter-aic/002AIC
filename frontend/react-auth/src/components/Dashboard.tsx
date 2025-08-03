/**
 * Dashboard Component with Role-Based Access Control
 * Demonstrates integration with the hybrid auth system
 */

import React, { useState, useEffect } from 'react';
import { useAuth, RequireRole, RequirePermission, createAuthenticatedClient } from '../auth/AuthProvider';

// API client
const apiClient = createAuthenticatedClient(process.env.REACT_APP_API_URL || 'http://localhost:8000');

interface Model {
  id: string;
  name: string;
  status: string;
  created_by: string;
  created_at: string;
}

interface Dataset {
  id: string;
  name: string;
  size: number;
  created_by: string;
  created_at: string;
}

const Dashboard: React.FC = () => {
  const { user, logout, hasRole } = useAuth();
  const [models, setModels] = useState<Model[]>([]);
  const [datasets, setDatasets] = useState<Dataset[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Load dashboard data
  useEffect(() => {
    const loadDashboardData = async () => {
      try {
        setLoading(true);
        
        // Load models if user has permission
        try {
          const modelsResponse = await apiClient.get('/v1/models');
          if (modelsResponse.ok) {
            const modelsData = await modelsResponse.json();
            setModels(modelsData.models || []);
          }
        } catch (err) {
          console.warn('Could not load models:', err);
        }

        // Load datasets if user has permission
        try {
          const datasetsResponse = await apiClient.get('/v1/datasets');
          if (datasetsResponse.ok) {
            const datasetsData = await datasetsResponse.json();
            setDatasets(datasetsData.datasets || []);
          }
        } catch (err) {
          console.warn('Could not load datasets:', err);
        }

      } catch (err) {
        setError('Failed to load dashboard data');
        console.error('Dashboard load error:', err);
      } finally {
        setLoading(false);
      }
    };

    loadDashboardData();
  }, []);

  // Create new model
  const createModel = async () => {
    try {
      const response = await apiClient.post('/v1/models', {
        name: `New Model ${Date.now()}`,
        description: 'Created from dashboard',
      });

      if (response.ok) {
        const newModel = await response.json();
        setModels(prev => [...prev, newModel]);
      } else {
        setError('Failed to create model');
      }
    } catch (err) {
      setError('Failed to create model');
      console.error('Model creation error:', err);
    }
  };

  // Train model
  const trainModel = async (modelId: string) => {
    try {
      const response = await apiClient.post(`/v1/models/${modelId}/train`, {
        training_config: {
          epochs: 10,
          batch_size: 32,
        },
      });

      if (response.ok) {
        // Update model status
        setModels(prev => prev.map(model => 
          model.id === modelId 
            ? { ...model, status: 'training' }
            : model
        ));
      } else {
        setError('Failed to start training');
      }
    } catch (err) {
      setError('Failed to start training');
      console.error('Training error:', err);
    }
  };

  if (loading) {
    return (
      <div className="dashboard-loading">
        <h2>Loading Dashboard...</h2>
        <div className="spinner"></div>
      </div>
    );
  }

  return (
    <div className="dashboard">
      {/* Header */}
      <header className="dashboard-header">
        <div className="user-info">
          <h1>002AIC Platform Dashboard</h1>
          <div className="user-details">
            <span>Welcome, {user?.firstName} {user?.lastName}</span>
            <span className="user-roles">
              Roles: {user?.roles.join(', ')}
            </span>
          </div>
        </div>
        <button onClick={logout} className="logout-btn">
          Logout
        </button>
      </header>

      {error && (
        <div className="error-banner">
          {error}
          <button onClick={() => setError(null)}>×</button>
        </div>
      )}

      {/* Main Content */}
      <main className="dashboard-content">
        
        {/* Models Section */}
        <RequirePermission resource="model" action="read">
          <section className="dashboard-section">
            <div className="section-header">
              <h2>AI Models</h2>
              <RequirePermission resource="model" action="create">
                <button onClick={createModel} className="create-btn">
                  Create Model
                </button>
              </RequirePermission>
            </div>
            
            <div className="models-grid">
              {models.length === 0 ? (
                <div className="empty-state">
                  <p>No models found. Create your first model to get started.</p>
                </div>
              ) : (
                models.map(model => (
                  <div key={model.id} className="model-card">
                    <h3>{model.name}</h3>
                    <p>Status: <span className={`status ${model.status}`}>{model.status}</span></p>
                    <p>Created by: {model.created_by}</p>
                    <p>Created: {new Date(model.created_at).toLocaleDateString()}</p>
                    
                    <div className="model-actions">
                      <RequirePermission resource="model" action="train">
                        <button 
                          onClick={() => trainModel(model.id)}
                          disabled={model.status === 'training'}
                          className="train-btn"
                        >
                          {model.status === 'training' ? 'Training...' : 'Train'}
                        </button>
                      </RequirePermission>
                      
                      <RequirePermission resource="model" action="deploy">
                        <button className="deploy-btn">Deploy</button>
                      </RequirePermission>
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>
        </RequirePermission>

        {/* Datasets Section */}
        <RequirePermission resource="dataset" action="read">
          <section className="dashboard-section">
            <div className="section-header">
              <h2>Datasets</h2>
              <RequirePermission resource="dataset" action="create">
                <button className="create-btn">Upload Dataset</button>
              </RequirePermission>
            </div>
            
            <div className="datasets-grid">
              {datasets.length === 0 ? (
                <div className="empty-state">
                  <p>No datasets found. Upload your first dataset to get started.</p>
                </div>
              ) : (
                datasets.map(dataset => (
                  <div key={dataset.id} className="dataset-card">
                    <h3>{dataset.name}</h3>
                    <p>Size: {(dataset.size / 1024 / 1024).toFixed(2)} MB</p>
                    <p>Created by: {dataset.created_by}</p>
                    <p>Created: {new Date(dataset.created_at).toLocaleDateString()}</p>
                    
                    <div className="dataset-actions">
                      <button className="view-btn">View</button>
                      <RequirePermission resource="dataset" action="update">
                        <button className="edit-btn">Edit</button>
                      </RequirePermission>
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>
        </RequirePermission>

        {/* Admin Section */}
        <RequireRole role="admin">
          <section className="dashboard-section admin-section">
            <h2>Administration</h2>
            <div className="admin-grid">
              <div className="admin-card">
                <h3>User Management</h3>
                <p>Manage platform users and permissions</p>
                <button className="admin-btn">Manage Users</button>
              </div>
              
              <div className="admin-card">
                <h3>System Monitoring</h3>
                <p>View system health and performance metrics</p>
                <button className="admin-btn">View Metrics</button>
              </div>
              
              <div className="admin-card">
                <h3>Security Audit</h3>
                <p>Review security logs and access patterns</p>
                <button className="admin-btn">View Audit Logs</button>
              </div>
            </div>
          </section>
        </RequireRole>

        {/* Developer Tools */}
        <RequireRole role="developer">
          <section className="dashboard-section developer-section">
            <h2>Developer Tools</h2>
            <div className="developer-grid">
              <div className="tool-card">
                <h3>API Explorer</h3>
                <p>Test and explore platform APIs</p>
                <button className="tool-btn">Open API Explorer</button>
              </div>
              
              <div className="tool-card">
                <h3>Pipeline Builder</h3>
                <p>Create and manage ML pipelines</p>
                <button className="tool-btn">Build Pipeline</button>
              </div>
            </div>
          </section>
        </RequireRole>

        {/* Data Scientist Tools */}
        <RequireRole role="data-scientist">
          <section className="dashboard-section scientist-section">
            <h2>Data Science Tools</h2>
            <div className="scientist-grid">
              <div className="tool-card">
                <h3>Experiment Tracking</h3>
                <p>Track and compare ML experiments</p>
                <button className="tool-btn">View Experiments</button>
              </div>
              
              <div className="tool-card">
                <h3>Data Analysis</h3>
                <p>Analyze datasets and generate insights</p>
                <button className="tool-btn">Analyze Data</button>
              </div>
            </div>
          </section>
        </RequireRole>

      </main>
    </div>
  );
};

export default Dashboard;
