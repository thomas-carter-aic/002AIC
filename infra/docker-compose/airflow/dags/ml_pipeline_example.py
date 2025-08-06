"""
Nexus Platform - Example ML Pipeline DAG
This DAG demonstrates a complete ML pipeline workflow
"""

from datetime import datetime, timedelta
from airflow import DAG
from airflow.operators.python import PythonOperator
from airflow.operators.bash import BashOperator
import pandas as pd
import numpy as np
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import accuracy_score, classification_report
import mlflow
import mlflow.sklearn
import joblib
import os

# Default arguments for the DAG
default_args = {
    'owner': 'nexus-ai-team',
    'depends_on_past': False,
    'start_date': datetime(2024, 1, 1),
    'email_on_failure': False,
    'email_on_retry': False,
    'retries': 1,
    'retry_delay': timedelta(minutes=5),
}

# Create the DAG
dag = DAG(
    'nexus_ml_pipeline_example',
    default_args=default_args,
    description='Example ML Pipeline for Nexus Platform',
    schedule_interval=timedelta(days=1),
    catchup=False,
    tags=['ml', 'example', 'nexus'],
)

def extract_data(**context):
    """Extract data from various sources"""
    print("🔍 Extracting data...")
    
    # Simulate data extraction (replace with actual data sources)
    np.random.seed(42)
    n_samples = 1000
    n_features = 10
    
    # Generate synthetic dataset
    X = np.random.randn(n_samples, n_features)
    y = (X[:, 0] + X[:, 1] > 0).astype(int)
    
    # Create DataFrame
    feature_names = [f'feature_{i}' for i in range(n_features)]
    df = pd.DataFrame(X, columns=feature_names)
    df['target'] = y
    
    # Save to temporary location
    data_path = '/tmp/extracted_data.csv'
    df.to_csv(data_path, index=False)
    
    print(f"✅ Data extracted: {df.shape[0]} rows, {df.shape[1]} columns")
    return data_path

def validate_data(**context):
    """Validate data quality and integrity"""
    print("🔍 Validating data quality...")
    
    data_path = context['task_instance'].xcom_pull(task_ids='extract_data')
    df = pd.read_csv(data_path)
    
    # Data quality checks
    checks = {
        'no_missing_values': df.isnull().sum().sum() == 0,
        'correct_shape': df.shape[0] > 0 and df.shape[1] > 0,
        'target_distribution': df['target'].value_counts().min() > 10,
        'feature_variance': df.drop('target', axis=1).var().min() > 0.01
    }
    
    # Log validation results
    for check, passed in checks.items():
        status = "✅ PASS" if passed else "❌ FAIL"
        print(f"{status}: {check}")
    
    if not all(checks.values()):
        raise ValueError("Data validation failed!")
    
    print("✅ Data validation completed successfully")
    return data_path

def preprocess_data(**context):
    """Preprocess and feature engineer the data"""
    print("🔧 Preprocessing data...")
    
    data_path = context['task_instance'].xcom_pull(task_ids='validate_data')
    df = pd.read_csv(data_path)
    
    # Feature engineering
    df['feature_sum'] = df[[col for col in df.columns if col.startswith('feature_')]].sum(axis=1)
    df['feature_mean'] = df[[col for col in df.columns if col.startswith('feature_')]].mean(axis=1)
    
    # Split features and target
    X = df.drop('target', axis=1)
    y = df['target']
    
    # Train-test split
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )
    
    # Save preprocessed data
    train_path = '/tmp/train_data.csv'
    test_path = '/tmp/test_data.csv'
    
    train_df = pd.concat([X_train, y_train], axis=1)
    test_df = pd.concat([X_test, y_test], axis=1)
    
    train_df.to_csv(train_path, index=False)
    test_df.to_csv(test_path, index=False)
    
    print(f"✅ Data preprocessed: Train={X_train.shape}, Test={X_test.shape}")
    return {'train_path': train_path, 'test_path': test_path}

def train_model(**context):
    """Train the ML model"""
    print("🤖 Training model...")
    
    # Set MLflow tracking URI
    mlflow.set_tracking_uri("http://mlflow-server:5000")
    mlflow.set_experiment("nexus-ml-pipeline")
    
    paths = context['task_instance'].xcom_pull(task_ids='preprocess_data')
    train_df = pd.read_csv(paths['train_path'])
    
    X_train = train_df.drop('target', axis=1)
    y_train = train_df['target']
    
    with mlflow.start_run(run_name=f"training_{datetime.now().strftime('%Y%m%d_%H%M%S')}"):
        # Model parameters
        params = {
            'n_estimators': 100,
            'max_depth': 10,
            'random_state': 42
        }
        
        # Log parameters
        mlflow.log_params(params)
        
        # Train model
        model = RandomForestClassifier(**params)
        model.fit(X_train, y_train)
        
        # Log model
        mlflow.sklearn.log_model(model, "model")
        
        # Save model locally for next task
        model_path = '/tmp/trained_model.joblib'
        joblib.dump(model, model_path)
        
        print("✅ Model training completed")
        return model_path

def evaluate_model(**context):
    """Evaluate the trained model"""
    print("📊 Evaluating model...")
    
    # Set MLflow tracking URI
    mlflow.set_tracking_uri("http://mlflow-server:5000")
    
    model_path = context['task_instance'].xcom_pull(task_ids='train_model')
    paths = context['task_instance'].xcom_pull(task_ids='preprocess_data')
    
    # Load model and test data
    model = joblib.load(model_path)
    test_df = pd.read_csv(paths['test_path'])
    
    X_test = test_df.drop('target', axis=1)
    y_test = test_df['target']
    
    # Make predictions
    y_pred = model.predict(X_test)
    y_pred_proba = model.predict_proba(X_test)
    
    # Calculate metrics
    accuracy = accuracy_score(y_test, y_pred)
    
    with mlflow.start_run(run_name=f"evaluation_{datetime.now().strftime('%Y%m%d_%H%M%S')}"):
        # Log metrics
        mlflow.log_metric("accuracy", accuracy)
        mlflow.log_metric("test_samples", len(y_test))
        
        # Log classification report
        report = classification_report(y_test, y_pred, output_dict=True)
        for label, metrics in report.items():
            if isinstance(metrics, dict):
                for metric, value in metrics.items():
                    mlflow.log_metric(f"{label}_{metric}", value)
    
    print(f"✅ Model evaluation completed - Accuracy: {accuracy:.4f}")
    
    # Model validation threshold
    if accuracy < 0.7:
        raise ValueError(f"Model accuracy {accuracy:.4f} below threshold 0.7")
    
    return {'accuracy': accuracy, 'model_path': model_path}

def deploy_model(**context):
    """Deploy the model to serving infrastructure"""
    print("🚀 Deploying model...")
    
    evaluation_results = context['task_instance'].xcom_pull(task_ids='evaluate_model')
    model_path = evaluation_results['model_path']
    accuracy = evaluation_results['accuracy']
    
    # Set MLflow tracking URI
    mlflow.set_tracking_uri("http://mlflow-server:5000")
    
    with mlflow.start_run(run_name=f"deployment_{datetime.now().strftime('%Y%m%d_%H%M%S')}"):
        # Load and register model
        model = joblib.load(model_path)
        
        # Register model in MLflow Model Registry
        model_name = "nexus-example-classifier"
        mlflow.sklearn.log_model(
            model, 
            "model",
            registered_model_name=model_name
        )
        
        # Log deployment metadata
        mlflow.log_param("deployment_timestamp", datetime.now().isoformat())
        mlflow.log_param("model_accuracy", accuracy)
        mlflow.log_param("deployment_status", "success")
        
        print(f"✅ Model deployed successfully - {model_name} with accuracy {accuracy:.4f}")
    
    return model_name

# Define tasks
extract_task = PythonOperator(
    task_id='extract_data',
    python_callable=extract_data,
    dag=dag,
)

validate_task = PythonOperator(
    task_id='validate_data',
    python_callable=validate_data,
    dag=dag,
)

preprocess_task = PythonOperator(
    task_id='preprocess_data',
    python_callable=preprocess_data,
    dag=dag,
)

train_task = PythonOperator(
    task_id='train_model',
    python_callable=train_model,
    dag=dag,
)

evaluate_task = PythonOperator(
    task_id='evaluate_model',
    python_callable=evaluate_model,
    dag=dag,
)

deploy_task = PythonOperator(
    task_id='deploy_model',
    python_callable=deploy_model,
    dag=dag,
)

# Health check task
health_check_task = BashOperator(
    task_id='health_check',
    bash_command='curl -f http://health-check-service:8085/health || exit 1',
    dag=dag,
)

# Define task dependencies
extract_task >> validate_task >> preprocess_task >> train_task >> evaluate_task >> deploy_task
health_check_task >> extract_task
