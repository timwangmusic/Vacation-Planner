import React  from "react"
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import Col from 'react-bootstrap/Col'
import useForm from './useForm.js'
// Using hooks instead of a Class and constructor


const Login = () => {   
    const { values, handleChange, handleSubmit } = useForm(signup)  
    
    function signup(){
        console.log(values)  
    }

    var proxyUrl = 'https://cors-anywhere.herokuapp.com/',
        targetUrl = 'http://vacation-planner-v1.herokuapp.com/login'
    
    fetch ( proxyUrl+targetUrl, {
            method: 'POST',
            headers: {
		        'content-Type': 'application/json', 
		        'Accept' : 'application/json',
		    },
        body: JSON.stringify(values)
    })
    .then((response) => response.json())
    .then((values) => {
        console.log('Success:', values)
    })
    .catch((error) => {
        console.error('Error:', error)
    })
    
	
    return (
<Form noValidate onSubmit={handleSubmit}>
    <Form.Row>
        <Form.Group as={Col} controlId="formGridUserName">
            <Form.Label>UserName</Form.Label>
            <Form.Control
                autoFocus
                name={'username'}
                value={values.username}
                type='username'
                placeholder="username"
                onChange={handleChange}
            />
        </Form.Group>
        
        <Form.Group as={Col} controlId="formGridFirstName">
            <Form.Label>First Name</Form.Label>
            <Form.Control
            autoFocus
            name={'firstname'}
            value={values.firstname}
            type='first name' 
            placeholder="First name" 
            onChange={handleChange}
            />
        </Form.Group>

        <Form.Group as={Col} controlId="formGridSecondName">
            <Form.Label>Last Name</Form.Label>
            <Form.Control 
            autoFocus
            name={'lastname'}
            value={values.lastname}
            type='last name' 
            placeholder="Last name"
            onChange={handleChange}
            />
        </Form.Group>
    </Form.Row>

    <Form.Group controlId="formBasicEmail">
        <Form.Label>Email address</Form.Label>
        <Form.Control 
        autoFocus 
        type="email" 
        name={'email'}
        value={values.email} 
        onChange={handleChange}
        placeholder="Enter email" 
        />
        <Form.Text className="text-muted">
            We'll never share your email with anyone else.
        </Form.Text>
  </Form.Group>

  <Form.Group controlId="formBasicPassword">
    <Form.Label>Password</Form.Label>
    <Form.Control 
        autoFocus
        type="password"
        name={'password'}
        value={values.password}
        onChange={handleChange}
        placeholder="Password" />
    </Form.Group>
        <Form.Group controlId="formBasicCheckbox">
            <Form.Check type="checkbox" label="Check me out" />
    </Form.Group>
    
    <Button 
        variant="primary"
        type="submit"
        value="Submit"
        >
        Login
    </Button>

</Form>
  );
}

export default Login
