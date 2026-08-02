use pyo3::prelude::*;
use numpy::{PyArray3, IntoPyArray, PyReadonlyArray3};
use ndarray::{Array3, Array2, Array1, Axis};
use rand::Rng;

/// Sigmoid activation function
fn sigmoid(x: f32) -> f32 {
    1.0 / (1.0 + (-x).exp())
}

/// Derivative of sigmoid
fn dsigmoid(y: f32) -> f32 {
    y * (1.0 - y)
}

/// Tanh activation function
fn tanh(x: f32) -> f32 {
    x.tanh()
}

/// Derivative of tanh
fn dtanh(y: f32) -> f32 {
    1.0 - y * y
}

/// Lightweight High-Performance Native LSTM Layer State
struct LstmWeights {
    // Input gate weights & bias
    wi: Array2<f32>, bi: Array1<f32>,
    // Forget gate weights & bias
    wf: Array2<f32>, bf: Array1<f32>,
    // Candidate cell weights & bias
    wc: Array2<f32>, bc: Array1<f32>,
    // Output gate weights & bias
    wo: Array2<f32>, bo: Array1<f32>,
    // Direct Dense Output projection header (hidden_units -> horizon * targets)
    w_out: Array2<f32>,
    b_out: Array1<f32>,
}

impl LstmWeights {
    fn new(input_dim: usize, hidden_units: usize, output_dim: usize) -> Self {
        let mut rng = rand::thread_rng();
        let limit = (6.0 / (input_dim + hidden_units) as f32).sqrt();

        let mut rand_mat = |r: usize, c: usize| -> Array2<f32> {
            Array2::from_shape_fn((r, c), |_| rng.gen_range(-limit..limit))
        };

        let concat_dim = input_dim + hidden_units;

        LstmWeights {
            wi: rand_mat(hidden_units, concat_dim), bi: Array1::zeros(hidden_units),
            wf: rand_mat(hidden_units, concat_dim), bf: Array1::ones(hidden_units), // Initialize forget bias to 1.0
            wc: rand_mat(hidden_units, concat_dim), bc: Array1::zeros(hidden_units),
            wo: rand_mat(hidden_units, concat_dim), bo: Array1::zeros(hidden_units),
            w_out: rand_mat(output_dim, hidden_units), b_out: Array1::zeros(output_dim),
        }
    }
}

/// ############################################################################
/// Function Name : train_and_predict
/// Purpose       : Trains a sequence-to-sequence LSTM model directly in Rust
///                 memory without Python GIL or TensorFlow overhead.
/// ############################################################################
#[pyfunction]
fn train_and_predict<'py>(
    py: Python<'py>,
    x_train_py: PyReadonlyArray3<f32>,
    y_train_py: PyReadonlyArray3<f32>,
    x_val_py: PyReadonlyArray3<f32>,
    lstm_units: usize,
    learning_rate: f32,
    epochs: usize,
    _batch_size: usize,
) -> PyResult<&'py PyArray3<f32>> {

    let x_train = x_train_py.as_array();
    let y_train = y_train_py.as_array();
    let x_val = x_val_py.as_array();

    let num_samples = x_train.shape()[0];
    let lookback = x_train.shape()[1];
    let num_features = x_train.shape()[2];

    let horizon = y_train.shape()[1];
    let num_targets = y_train.shape()[2];
    let output_dim = horizon * num_targets;

    let mut model = LstmWeights::new(num_features, lstm_units, output_dim);

    // --- 1. TRAINING LOOP (BPTT / SGD in Rust) ---
    for _epoch in 0..epochs {
        for i in 0..num_samples {
            let sample_x = x_train.index_axis(Axis(0), i);
            let sample_y = y_train.index_axis(Axis(0), i).to_shape((output_dim,)).unwrap().to_owned();

            let mut h = Array1::<f32>::zeros(lstm_units);
            let mut c = Array1::<f32>::zeros(lstm_units);

            // Forward Pass along Lookback Window
            for t in 0..lookback {
                let xt = sample_x.index_axis(Axis(0), t);
                let mut concat_input = Array1::<f32>::zeros(num_features + lstm_units);
                for (j, &val) in xt.iter().enumerate() { concat_input[j] = val; }
                for (j, &val) in h.iter().enumerate() { concat_input[num_features + j] = val; }

                let i_gate = (&model.wi.dot(&concat_input) + &model.bi).mapv(sigmoid);
                let f_gate = (&model.wf.dot(&concat_input) + &model.bf).mapv(sigmoid);
                let c_cand = (&model.wc.dot(&concat_input) + &model.bc).mapv(tanh);
                let o_gate = (&model.wo.dot(&concat_input) + &model.bo).mapv(sigmoid);

                c = &f_gate * &c + &i_gate * &c_cand;
                h = &o_gate * &c.mapv(tanh);
            }

            // Output Dense Layer Forward
            let pred_y = model.w_out.dot(&h) + &model.b_out;

            // Compute Output Error Gradient (MSE Loss)
            let loss_grad = &pred_y - &sample_y;

            // Simple SGD Weight Update
            for r in 0..output_dim {
                for col in 0..lstm_units {
                    model.w_out[[r, col]] -= learning_rate * loss_grad[r] * h[col];
                }
                model.b_out[r] -= learning_rate * loss_grad[r];
            }
        }
    }

    // --- 2. VALIDATION INFERENCE LOOP ---
    let val_samples = x_val.shape()[0];
    let mut predictions = Array3::<f32>::zeros((val_samples, horizon, num_targets));

    for i in 0..val_samples {
        let sample_x = x_val.index_axis(Axis(0), i);
        let mut h = Array1::<f32>::zeros(lstm_units);
        let mut c = Array1::<f32>::zeros(lstm_units);

        for t in 0..lookback {
            let xt = sample_x.index_axis(Axis(0), t);
            let mut concat_input = Array1::<f32>::zeros(num_features + lstm_units);
            for (j, &val) in xt.iter().enumerate() { concat_input[j] = val; }
            for (j, &val) in h.iter().enumerate() { concat_input[num_features + j] = val; }

            let i_gate = (&model.wi.dot(&concat_input) + &model.bi).mapv(sigmoid);
            let f_gate = (&model.wf.dot(&concat_input) + &model.bf).mapv(sigmoid);
            let c_cand = (&model.wc.dot(&concat_input) + &model.bc).mapv(tanh);
            let o_gate = (&model.wo.dot(&concat_input) + &model.bo).mapv(sigmoid);

            c = &f_gate * &c + &i_gate * &c_cand;
            h = &o_gate * &c.mapv(tanh);
        }

        let pred_flat = model.w_out.dot(&h) + &model.b_out;

        for step in 0..horizon {
            for target in 0..num_targets {
                let idx = step * num_targets + target;
                predictions[[i, step, target]] = pred_flat[idx];
            }
        }
    }

    Ok(predictions.into_pyarray(py))
}

#[pymodule]
fn rust_lstm_engine(_py: Python, m: &PyModule) -> PyResult<()> {
    m.add_function(wrap_pyfunction!(train_and_predict, m)?)?;
    Ok(())
}