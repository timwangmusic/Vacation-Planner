import React, { useState } from "react"
import Form from 'react-bootstrap/Form'
import { Button, FormGroup, FormControl, FormLabel } from "react-bootstrap"
import "./Login.css"


function Login(props) {
	const [email, setEmail] = useState("");
	const [password, setPassword] = useState("");

	function validateForm() {
		return email.length > 0 && password.length > 0;
	}

	function handleSubmit(event) {
		event.preventDefault();
	}

	return (
	<div className="Login">
        <div class="modal-body">
            <form class="form-horizontal" role="form" onSubmit={handleSubmit}>
	            <div class="row">
                <div class="form-group">
                <label class="control-label col-xs-2">Title:</label>
                <div class="col-xs-10">
                    <input  type="text" class="form-control" data-bind="value: title" />
                </div>
                </div>
                </div>
        <div class="row">
            <div class="form-group">
                <label class="control-label col-xs-2">Start:</label>
                <div class="col-xs-4">
                   <div class="input-group">
                      <input  type="text" class="form-control" data-bind="value: title" />
                      <span class="input-group-addon"><i class="fa fa-calendar"></i></span>
                </div>
               </div>
            </div>
        </div>
  <Button variant="primary" type="submit">
    Submit
  </Button> 
            </form>
        </div>
        </div>
	);
}


export default Login
